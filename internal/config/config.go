// Package config centraliza la carga de configuración desde variables de entorno.
// Ningún secreto (API keys, tokens) debe aparecer en logs ni en valores por defecto.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config agrupa toda la configuración de core-engine.
type Config struct {
	// HTTP interno (API mínima de clientes/tareas).
	HTTPAddr string

	// Persistencia.
	DatabaseDSN string

	// llm-anonymizer interno.
	AnonymizerURL   string
	AnonymizerToken string

	// Jev.
	JevBaseURL string
	JevAPIKey  string

	// Extractores / LLM.
	OpenAIAPIKey  string
	OpenAIBaseURL string
	OpenAIModel   string

	// Clasificador.
	ClassifierMode  string
	ClassifierModel string

	// Gmail.
	GmailAccessToken  string
	GmailClientID     string
	GmailClientSecret string
	GmailRefreshToken string
	GmailQuery        string

	// Gmail push (Pub/Sub).
	GmailPushMode           string
	GmailPubSubTopic        string
	GmailPubSubSubscription string
	GmailPollFallback       time.Duration

	// Pre-filtro ligero.
	MinWords int

	// Umbrales de confianza.
	ExtractConfidence     float64
	ReviewBelowConfidence float64

	// Workers.
	WorkerConcurrency int

	// Timeouts.
	AnonymizerTimeout time.Duration
	JevTimeout        time.Duration
	ExtractorTimeout  time.Duration

	// Reintentos.
	MaxRetries int
}

// Load lee la configuración desde el entorno aplicando valores por defecto.
func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:                getenv("HTTP_ADDR", ":8081"),
		DatabaseDSN:             getenv("DATABASE_DSN", ""),
		AnonymizerURL:           getenv("ANONYMIZER_URL", "http://llm-anonymizer:8080"),
		AnonymizerToken:         getenv("ANONYMIZER_TOKEN", ""),
		JevBaseURL:              getenv("JEV_BASE_URL", "https://www.jevai.org"),
		JevAPIKey:               getenv("JEV_API_KEY", ""),
		OpenAIAPIKey:            getenv("OPENAI_API_KEY", ""),
		OpenAIBaseURL:           getenv("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		OpenAIModel:             getenv("OPENAI_MODEL", "gpt-4o-mini"),
		ClassifierMode:          getenv("CLASSIFIER_MODE", "chain"),
		ClassifierModel:         getenv("CLASSIFIER_MODEL", ""),
		GmailAccessToken:        getenv("GMAIL_ACCESS_TOKEN", ""),
		GmailClientID:           getenv("GMAIL_CLIENT_ID", ""),
		GmailClientSecret:       getenv("GMAIL_CLIENT_SECRET", ""),
		GmailRefreshToken:       getenv("GMAIL_REFRESH_TOKEN", ""),
		GmailQuery:              getenv("GMAIL_QUERY", ""),
		GmailPushMode:           getenv("GMAIL_PUSH_MODE", "poll"),
		GmailPubSubTopic:        getenv("GMAIL_PUBSUB_TOPIC", ""),
		GmailPubSubSubscription: getenv("GMAIL_PUBSUB_SUBSCRIPTION", ""),
		GmailPollFallback:       getenvDuration("GMAIL_POLL_FALLBACK", 5*time.Minute),
		MinWords:                getenvInt("MIN_WORDS", 3),
		ExtractConfidence:       getenvFloat("EXTRACT_CONFIDENCE", 0.6),
		ReviewBelowConfidence:   getenvFloat("REVIEW_BELOW_CONFIDENCE", 0.5),
		WorkerConcurrency:       getenvInt("WORKER_CONCURRENCY", 4),
		AnonymizerTimeout:       getenvDuration("ANONYMIZER_TIMEOUT", 10*time.Second),
		JevTimeout:              getenvDuration("JEV_TIMEOUT", 15*time.Second),
		ExtractorTimeout:        getenvDuration("EXTRACTOR_TIMEOUT", 30*time.Second),
		MaxRetries:              getenvInt("MAX_RETRIES", 2),
	}

	if strings.TrimSpace(cfg.DatabaseDSN) == "" {
		// Sin DSN se usa un almacén en memoria (desarrollo/demo).
		return cfg, nil
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getenvFloat(key string, fallback float64) float64 {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
