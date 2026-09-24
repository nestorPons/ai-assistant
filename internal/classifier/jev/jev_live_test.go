package jev

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nestorPons/ai-assistant/internal/classifier"
)

func TestLiveJevAPI(t *testing.T) {
	if os.Getenv("JEV_LIVE_TEST") == "" {
		t.Skip("definir JEV_LIVE_TEST=1 para ejecutar contra la API real de Jev")
	}

	baseURL := os.Getenv("JEV_BASE_URL")
	apiKey := os.Getenv("JEV_API_KEY")
	if baseURL == "" || apiKey == "" {
		env := loadDotEnv(filepath.Join("..", "..", "..", ".env"))
		if baseURL == "" {
			baseURL = env["JEV_BASE_URL"]
		}
		if apiKey == "" {
			apiKey = env["JEV_API_KEY"]
		}
	}
	if baseURL == "" {
		baseURL = "https://www.jevai.org"
	}
	if apiKey == "" {
		t.Fatal("JEV_API_KEY no definido (ni en entorno ni en .env)")
	}

	p := New(baseURL, apiKey, 20*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	res, err := p.Classify(ctx, classifier.ClassificationInput{
		CleanPrompt: "Agenda una reunión con el cliente Acme el martes a las 10:00",
	})
	if err != nil {
		t.Fatalf("la API de Jev falló: %v", err)
	}

	t.Logf("decision=%s confidence=%.2f review=%v blocked=%v reason=%q",
		res.Decision, res.Confidence, res.NeedsReview, res.Blocked, res.Reason)

	switch res.Decision {
	case classifier.DecisionDBAction, classifier.DecisionExtract, classifier.DecisionDiscard:
	default:
		t.Fatalf("decisión inesperada: %q", res.Decision)
	}
	if res.Decision == classifier.DecisionDBAction && res.Action == nil {
		t.Error("db_action sin action")
	}
}

func loadDotEnv(path string) map[string]string {
	out := map[string]string{}
	f, err := os.Open(path)
	if err != nil {
		return out
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
		out[key] = val
	}
	return out
}
