package classifier_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nestorPons/ai-assistant/internal/classifier"
	"github.com/nestorPons/ai-assistant/internal/classifier/jev"
	"github.com/nestorPons/ai-assistant/internal/classifier/remote"
	"github.com/nestorPons/ai-assistant/internal/llm"
	"github.com/nestorPons/ai-assistant/internal/llm/openai"
)

type namedClassifier struct {
	name string
	cls  classifier.Classifier
}

func TestLiveClassifiers(t *testing.T) {
	if os.Getenv("CLASSIFIER_LIVE_TEST") == "" {
		t.Skip("definir CLASSIFIER_LIVE_TEST=1 para probar los clasificadores reales")
	}

	env := loadDotEnv(filepath.Join("..", "..", ".env"))
	get := func(key string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return env[key]
	}

	var classifiers []namedClassifier

	if key := get("JEV_API_KEY"); key != "" {
		base := get("JEV_BASE_URL")
		if base == "" {
			base = "https://www.jevai.org"
		}
		classifiers = append(classifiers, namedClassifier{"jev", jev.New(base, key, 30*time.Second)})
	}

	if key := get("OPENAI_API_KEY"); key != "" {
		model := firstNonEmpty(get("CLASSIFIER_MODEL"), get("OPENAI_MODEL"), "gpt-4o-mini")
		base := firstNonEmpty(get("OPENAI_BASE_URL"), "https://api.openai.com/v1")
		classifiers = append(classifiers, namedClassifier{"llm", remote.New(openai.New(key, base), model, nil)})
	}

	if len(classifiers) == 0 {
		t.Fatal("sin credenciales: definir JEV_API_KEY y/o OPENAI_API_KEY en .env")
	}

	cases := []struct {
		name     string
		text     string
		wantWork bool
	}{
		{
			name:     "charla_casual",
			text:     "Hola, ¿cómo estás? Hoy hace un día soleado y estaba pensando en ir al cine con unos amigos.",
			wantWork: false,
		},
		{
			name:     "peticion_trabajo",
			text:     "Hola, necesito que prepares un informe de ventas del mes pasado y me lo entregues el viernes a más tardar.",
			wantWork: true,
		},
	}

	for _, nc := range classifiers {
		for _, tc := range cases {
			t.Run(nc.name+"/"+tc.name, func(t *testing.T) {
				res, err := classifyWithRetry(t, nc.cls, classifier.ClassificationInput{CleanPrompt: tc.text})
				if err != nil {
					var le *llm.Error
					if errors.As(err, &le) && le.Retryable() {
						t.Skipf("clasificador %s no disponible (retryable): %v", nc.name, err)
					}
					t.Fatalf("classify falló: %v", err)
				}

				if out, err := json.MarshalIndent(res, "", "  "); err == nil {
					t.Logf("resultado JSON:\n%s", out)
				}

				if tc.wantWork {
					if res.Decision != classifier.DecisionExtract {
						t.Errorf("texto de trabajo: decision = %q, want %q (review=%v blocked=%v reason=%q)",
							res.Decision, classifier.DecisionExtract, res.NeedsReview, res.Blocked, res.Reason)
					}
					if res.NeedsReview {
						t.Errorf("texto de trabajo no debería requerir revisión: %q", res.Reason)
					}
					if res.Blocked {
						t.Errorf("texto de trabajo no debería estar bloqueado: %q", res.Reason)
					}
				} else {
					if res.Decision == classifier.DecisionExtract {
						t.Errorf("texto casual no debería clasificarse como tarea (reason=%q)", res.Reason)
					}
				}
			})
		}
	}
}

func classifyWithRetry(t *testing.T, cls classifier.Classifier, input classifier.ClassificationInput) (classifier.ClassificationResult, error) {
	t.Helper()
	var (
		res classifier.ClassificationResult
		err error
	)
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<attempt) * time.Second)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		res, err = cls.Classify(ctx, input)
		cancel()
		if err == nil {
			return res, nil
		}
		var le *llm.Error
		if errors.As(err, &le) && le.Retryable() {
			t.Logf("reintento %d por error retryable: %v", attempt+1, err)
			continue
		}
		return res, err
	}
	return res, err
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
		out[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(val), `"'`)
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
