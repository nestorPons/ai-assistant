// Command gmail-sync ejecuta uno o varios ciclos de sincronización incremental
// de Gmail a demanda (sin levantar core-engine). Guarda el cursor en un archivo
// local para poder correrlo cuando quieras.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/nestorPons/ai-assistant/internal/domain"
	"github.com/nestorPons/ai-assistant/internal/ingestion/gmail"
	"github.com/nestorPons/ai-assistant/internal/ingestion/pubsub"
	"github.com/nestorPons/ai-assistant/internal/oauth"
	"github.com/nestorPons/ai-assistant/internal/persistence"
)

func main() {
	envPath := flag.String("env", ".env", "archivo .env con credenciales")
	cursorPath := flag.String("cursor", ".gmail_cursor.json", "archivo local del cursor")
	full := flag.Bool("full", false, "en el primer arranque procesar los no leídos actuales")
	reset := flag.Bool("reset", false, "borrar el cursor guardado y volver a baseline")
	loop := flag.Bool("loop", false, "repetir la sincronización indefinidamente")
	interval := flag.Duration("interval", 30*time.Second, "intervalo entre ciclos en -loop")
	pubsubMode := flag.Bool("pubsub", false, "hacer un pull de Pub/Sub y luego sincronizar")
	subscription := flag.String("subscription", "", "suscripción pull (por defecto GMAIL_PUBSUB_SUBSCRIPTION)")
	watch := flag.Bool("watch", false, "registrar el watch de Gmail hacia el topic (requiere GMAIL_PUBSUB_TOPIC)")
	noAck := flag.Bool("no-ack", false, "no hacer ack de los mensajes de Pub/Sub")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	loadDotEnv(*envPath)

	if *reset {
		if err := os.Remove(*cursorPath); err != nil && !os.IsNotExist(err) {
			logger.Error("no se pudo borrar el cursor", "error", err)
			os.Exit(1)
		}
		logger.Info("cursor reiniciado")
	}

	clientID := os.Getenv("GMAIL_CLIENT_ID")
	clientSecret := os.Getenv("GMAIL_CLIENT_SECRET")
	refresh := os.Getenv("GMAIL_REFRESH_TOKEN")
	access := os.Getenv("GMAIL_ACCESS_TOKEN")

	var tokens *oauth.TokenSource
	switch {
	case clientID != "" && clientSecret != "" && refresh != "":
		tokens = oauth.New(oauth.Config{ClientID: clientID, ClientSecret: clientSecret, RefreshToken: refresh})
	case access != "":
		tokens = oauth.New(oauth.Config{AccessToken: access})
	default:
		logger.Error("faltan credenciales Gmail (GMAIL_CLIENT_ID/SECRET/REFRESH_TOKEN o GMAIL_ACCESS_TOKEN)")
		os.Exit(1)
	}

	cfg := gmail.Config{
		Tokens:    tokens,
		Query:     os.Getenv("GMAIL_QUERY"),
		TopicName: os.Getenv("GMAIL_PUBSUB_TOPIC"),
		Backlog:   *full,
		Sync:      fileCursor{path: *cursorPath},
		Logger:    logger,
	}
	src := gmail.New(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	out := make(chan domain.IncomingMessage, 1000)

	if *watch {
		if cfg.TopicName == "" {
			logger.Error("falta GMAIL_PUBSUB_TOPIC para -watch")
			os.Exit(1)
		}
		if err := src.EnsureWatch(ctx); err != nil {
			logger.Error("watch", "error", err)
			os.Exit(1)
		}
		logger.Info("watch registrado", "topic", cfg.TopicName)
	}

	if *loop {
		go func() {
			for m := range out {
				printMessage(m)
			}
		}()
		for {
			if err := src.Sync(ctx, out); err != nil {
				if ctx.Err() != nil {
					return
				}
				logger.Error("sync", "error", err)
			}
			logger.Info("ciclo completado", "espera", interval.String())
			select {
			case <-ctx.Done():
				return
			case <-time.After(*interval):
			}
		}
	}

	var (
		pc       *pubsub.Client
		sub      string
		pullMsgs []pubsub.ReceivedMessage
	)
	if *pubsubMode {
		sub = *subscription
		if sub == "" {
			sub = os.Getenv("GMAIL_PUBSUB_SUBSCRIPTION")
		}
		if sub == "" {
			logger.Error("falta -subscription o GMAIL_PUBSUB_SUBSCRIPTION")
			os.Exit(1)
		}
		pc = pubsub.New(tokens)

		msgs, err := pc.PullOnce(ctx, sub, 10)
		if err != nil {
			logger.Error("pubsub pull", "error", err)
			os.Exit(1)
		}
		pullMsgs = msgs
		logger.Info("notificaciones pubsub", "recibidas", len(msgs), "suscripcion", sub)
		for _, m := range msgs {
			logger.Info("notificación", "email", m.Notification.EmailAddress, "history_id", m.Notification.HistoryID)
		}
		if len(msgs) == 0 {
			logger.Info("sin notificaciones pendientes")
		}
	}

	if err := src.Sync(ctx, out); err != nil {
		logger.Error("sync", "error", err)
		os.Exit(1)
	}
	close(out)

	count := 0
	for m := range out {
		printMessage(m)
		count++
	}
	logger.Info("sync completado", "mensajes_nuevos", count)

	if pc != nil && !*noAck && len(pullMsgs) > 0 {
		ackIDs := make([]string, 0, len(pullMsgs))
		for _, m := range pullMsgs {
			ackIDs = append(ackIDs, m.AckID)
		}
		if err := pc.Ack(ctx, sub, ackIDs); err != nil {
			logger.Error("pubsub ack", "error", err)
		} else {
			logger.Info("pubsub ack", "confirmados", len(ackIDs))
		}
	}
}

func printMessage(m domain.IncomingMessage) {
	fmt.Printf("  [msg] id=%s from=%s name=%q content=%q\n", m.ID, m.ClientIdentifier, m.ClientName, m.RawContent)
}

// fileCursor implementa gmail.CursorStore sobre un archivo JSON local.
type fileCursor struct {
	path string
}

func (f fileCursor) GetSyncState(_ context.Context, source domain.Source) (*domain.SyncState, error) {
	data, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, persistence.ErrNotFound
		}
		return nil, err
	}
	var st domain.SyncState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	if st.Source == "" {
		st.Source = source
	}
	return &st, nil
}

func (f fileCursor) SaveSyncState(_ context.Context, st *domain.SyncState) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(f.path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(f.path, data, 0o600)
}

func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
}
