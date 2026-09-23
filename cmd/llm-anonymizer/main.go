// Command llm-anonymizer es el microservicio interno de ofuscación de PII.
// Compila como binario estático y se ejecuta dentro de la red interna.
package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/nestorPons/ai-assistant/internal/anonymizer"
	"github.com/nestorPons/ai-assistant/internal/anonymizer/detect"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	addr := getenv("ANONYMIZER_ADDR", ":8080")
	token := getenv("ANONYMIZER_TOKEN", "")
	maxBytes := getenvInt("ANONYMIZER_MAX_BYTES", 32<<10)
	ttl := getenvDuration("ANONYMIZER_TTL", 5*time.Minute)

	detector := detect.New(defaultPersons(), defaultOrgs(), defaultLocs())
	engine := anonymizer.NewEngine(detector, detect.NoopNER{})

	store := anonymizer.NewMemoryMappingStore()
	server := anonymizer.NewServer(engine, store, anonymizer.Config{
		Token:         token,
		MaxBytes:      int64(maxBytes),
		ProcessingTTL: ttl,
	}, logger)

	logger.Info("llm-anonymizer iniciado", "addr", addr)
	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		logger.Error("servidor detenido", "error", err)
		os.Exit(1)
	}
}

func defaultPersons() []string {
	return []string{"María", "Juan", "Pedro", "Ana", "Luis", "Carmen", "José", "Lucía"}
}

func defaultOrgs() []string {
	return []string{"Acme", "Globex", "Initech"}
}

func defaultLocs() []string {
	return []string{"Madrid", "Barcelona", "Valencia", "Sevilla", "Bilbao"}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n := 0
	for _, r := range v {
		if r < '0' || r > '9' {
			return fallback
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
