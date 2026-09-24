// Command core-engine es el orquestador principal: escucha canales, filtra
// usuarios autorizados, ofusca PII, clasifica con Jev y extrae tareas con un LLM.
package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nestorPons/ai-assistant/internal/anonymizer"
	"github.com/nestorPons/ai-assistant/internal/anonymizer/detect"
	"github.com/nestorPons/ai-assistant/internal/classifier"
	"github.com/nestorPons/ai-assistant/internal/classifier/jev"
	"github.com/nestorPons/ai-assistant/internal/classifier/remote"
	"github.com/nestorPons/ai-assistant/internal/config"
	"github.com/nestorPons/ai-assistant/internal/domain"
	"github.com/nestorPons/ai-assistant/internal/extractor"
	"github.com/nestorPons/ai-assistant/internal/httpapi"
	"github.com/nestorPons/ai-assistant/internal/ingestion"
	"github.com/nestorPons/ai-assistant/internal/ingestion/gmail"
	"github.com/nestorPons/ai-assistant/internal/ingestion/pubsub"
	"github.com/nestorPons/ai-assistant/internal/ingestion/simulated"
	"github.com/nestorPons/ai-assistant/internal/llm/openai"
	"github.com/nestorPons/ai-assistant/internal/oauth"
	"github.com/nestorPons/ai-assistant/internal/persistence"
	"github.com/nestorPons/ai-assistant/internal/pipeline"
	"github.com/nestorPons/ai-assistant/internal/worker"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuración inválida", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Persistencia: MariaDB o memoria (demo).
	var store persistence.Store
	if cfg.DatabaseDSN != "" {
		ms, err := persistence.NewMySQLStore(ctx, cfg.DatabaseDSN)
		if err != nil {
			logger.Error("conexión a MariaDB", "error", err)
			os.Exit(1)
		}
		defer ms.Close()
		store = ms
		logger.Info("persistencia conectada (MariaDB)")
	} else {
		store = persistence.NewMemoryStore()
		logger.Warn("sin DATABASE_DSN, usando almacén en memoria (demo)")
	}

	// Clasificador (Jev / LLM remoto / cadena de fallback).
	cls := buildClassifier(cfg, logger)

	// Extractor (OpenAI real o mock).
	var ext extractor.Extractor
	if cfg.OpenAIAPIKey != "" {
		ext = extractor.NewLLMExtractor(openai.New(cfg.OpenAIAPIKey, cfg.OpenAIBaseURL), cfg.OpenAIModel)
		logger.Info("extractor: OpenAI", "model", cfg.OpenAIModel)
	} else {
		ext = extractor.NewMockExtractor()
		logger.Warn("sin OPENAI_API_KEY, usando extractor simulado (demo)")
	}

	anonURL := cfg.AnonymizerURL
	if cfg.DatabaseDSN == "" {
		// Demo autocontenido: se arranca un anonimizador embebido en local.
		anonURL = startEmbeddedAnonymizer(logger)
	}

	anon := anonymizer.NewClient(anonURL, cfg.AnonymizerToken, cfg.AnonymizerTimeout)

	// Canal de entrada.
	sources, pubsubRunner := buildSources(cfg, store)

	// En modo demo (sin canal real) se da de alta un cliente autorizado.
	if cfg.DatabaseDSN == "" {
		seedDemoClient(store)
	}

	pipe := pipeline.New(store, anon, cls, ext, pipeline.Config{
		MinWords:              cfg.MinWords,
		ExtractConfidence:     cfg.ExtractConfidence,
		ReviewBelowConfidence: cfg.ReviewBelowConfidence,
	}, logger)

	wk := worker.New(pipe, cfg.WorkerConcurrency, cfg.MaxRetries, logger)

	// API mínima.
	api := httpapi.New(store, logger)
	go func() {
		logger.Info("api escuchando", "addr", cfg.HTTPAddr)
		if err := http.ListenAndServe(cfg.HTTPAddr, api.Handler()); err != nil {
			logger.Error("api detenida", "error", err)
		}
	}()

	// Workers e ingestión.
	input := make(chan domain.IncomingMessage, 100)
	go wk.Run(ctx, input)

	logger.Info("core-engine iniciado")
	for _, src := range sources {
		go func(s ingestion.Source) {
			logger.Info("fuente iniciada", "source", s.Name())
			if err := s.Start(ctx, input); err != nil && ctx.Err() == nil {
				logger.Error("fuente detenida", "source", s.Name(), "error", err)
			}
		}(src)
	}
	if pubsubRunner != nil {
		go func() {
			logger.Info("worker pubsub iniciado")
			if err := pubsubRunner(ctx, input); err != nil && ctx.Err() == nil {
				logger.Error("worker pubsub detenido", "error", err)
			}
		}()
	}

	<-ctx.Done()
	logger.Info("deteniendo core-engine")
	time.Sleep(100 * time.Millisecond)
}

func buildClassifier(cfg config.Config, logger *slog.Logger) classifier.Classifier {
	var jevCls, llmCls classifier.Classifier

	if cfg.JevAPIKey != "" {
		jevCls = jev.New(cfg.JevBaseURL, cfg.JevAPIKey, cfg.JevTimeout)
	}

	if cfg.OpenAIAPIKey != "" {
		model := cfg.ClassifierModel
		if model == "" {
			model = cfg.OpenAIModel
		}
		llmCls = remote.New(openai.New(cfg.OpenAIAPIKey, cfg.OpenAIBaseURL), model, logger)
	}

	rules := classifier.NewMockRuleClassifier()

	switch cfg.ClassifierMode {
	case "jev":
		if jevCls != nil {
			logger.Info("clasificador: Jev")
			return jevCls
		}
		logger.Warn("CLASSIFIER_MODE=jev sin JEV_API_KEY, usando reglas (demo)")
		return rules

	case "llm":
		if llmCls != nil {
			logger.Info("clasificador: LLM remoto", "model", cfg.ClassifierModel)
			return llmCls
		}
		logger.Warn("CLASSIFIER_MODE=llm sin OPENAI_API_KEY, usando reglas (demo)")
		return rules

	default:
		stages := make([]classifier.Classifier, 0, 3)
		if jevCls != nil {
			stages = append(stages, jevCls)
		}
		if llmCls != nil {
			stages = append(stages, llmCls)
		}
		stages = append(stages, rules)
		logger.Info("clasificador: cadena", "etapas", len(stages),
			"jev", jevCls != nil, "llm", llmCls != nil)
		return classifier.NewChain(logger, stages...)
	}
}

func buildSources(cfg config.Config, store persistence.Store) ([]ingestion.Source, func(context.Context, chan<- domain.IncomingMessage) error) {
	var sources []ingestion.Source
	var runner func(context.Context, chan<- domain.IncomingMessage) error

	if cfg.JevAPIKey != "" || cfg.OpenAIAPIKey != "" {
		// Con credenciales reales se usa Gmail como canal principal.
		hasRefresh := cfg.GmailClientID != "" && cfg.GmailClientSecret != "" && cfg.GmailRefreshToken != ""
		if cfg.GmailAccessToken != "" || hasRefresh {
			tokens := oauth.New(oauth.Config{
				AccessToken:  cfg.GmailAccessToken,
				ClientID:     cfg.GmailClientID,
				ClientSecret: cfg.GmailClientSecret,
				RefreshToken: cfg.GmailRefreshToken,
			})

			gcfg := gmail.Config{
				Tokens: tokens,
				Query:  cfg.GmailQuery,
				Sync:   store,
				Logger: slog.Default(),
			}

			pull := cfg.GmailPushMode == "pull" && cfg.GmailPubSubTopic != "" && cfg.GmailPubSubSubscription != ""
			if pull {
				gcfg.TopicName = cfg.GmailPubSubTopic
				gcfg.Fallback = cfg.GmailPollFallback
			}

			gs := gmail.New(gcfg)
			sources = append(sources, gs)

			if pull {
				pc := pubsub.New(tokens)
				runner = func(ctx context.Context, out chan<- domain.IncomingMessage) error {
					return pc.Run(ctx, cfg.GmailPubSubSubscription, func(ctx context.Context) error {
						return gs.Sync(ctx, out)
					})
				}
			}
		}
	}

	if len(sources) == 0 {
		// Sin canal real se usa la fuente simulada para validar el flujo.
		sources = append(sources, simulated.NewDefault())
	}
	return sources, runner
}

// seedDemoClient da de alta un cliente autorizado para la fuente simulada.
func seedDemoClient(store persistence.Store) {
	c := &domain.Client{
		Source:     domain.SourceGmail,
		Identifier: "maria@example.com",
		Name:       "María García",
		Tracked:    true,
		Active:     true,
	}
	if err := store.CreateClient(context.Background(), c); err != nil && err != persistence.ErrDuplicate {
		slog.Warn("no se pudo crear el cliente demo", "error", err)
	}
}

// startEmbeddedAnonymizer arranca un anonimizador en proceso y devuelve su URL.
func startEmbeddedAnonymizer(logger *slog.Logger) string {
	detector := detect.New(nil, nil, nil)
	engine := anonymizer.NewEngine(detector, detect.NoopNER{})
	store := anonymizer.NewMemoryMappingStore()
	srv := anonymizer.NewServer(engine, store, anonymizer.Config{}, logger)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		logger.Error("no se pudo arrancar el anonimizador embebido", "error", err)
		os.Exit(1)
	}
	go func() {
		if err := http.Serve(ln, srv.Handler()); err != nil {
			logger.Error("anonimizador embebido detenido", "error", err)
		}
	}()
	logger.Info("anonimizador embebido activo", "addr", ln.Addr().String())
	return "http://" + ln.Addr().String()
}
