package anonymizer

import (
	"strings"
	"testing"

	"github.com/nestorPons/ai-assistant/internal/anonymizer/detect"
)

func newTestEngine() *Engine {
	d := detect.New([]string{"María", "Juan"}, []string{"Acme"}, []string{"Madrid"})
	return NewEngine(d, detect.NoopNER{})
}

func TestSanitizeReplacesPII(t *testing.T) {
	e := newTestEngine()
	clean, entities, mapping := e.Sanitize("María escribe a maria@example.com desde Madrid")

	if strings.Contains(clean, "María") {
		t.Errorf("no ofuscó el nombre: %q", clean)
	}
	if strings.Contains(clean, "maria@example.com") {
		t.Errorf("no ofuscó el email: %q", clean)
	}
	if !strings.Contains(clean, "{{PERSONA_1}}") {
		t.Errorf("falta marcador de persona: %q", clean)
	}
	if !strings.Contains(clean, "{{EMAIL_1}}") {
		t.Errorf("falta marcador de email: %q", clean)
	}

	if mapping["{{EMAIL_1}}"] != "maria@example.com" {
		t.Errorf("mapping email incorrecto: %v", mapping)
	}
	if len(entities) == 0 {
		t.Error("no devolvió entidades")
	}
}

func TestSanitizeStableTokens(t *testing.T) {
	e := newTestEngine()
	clean, _, mapping := e.Sanitize("María y María se reúnen")

	count := strings.Count(clean, "{{PERSONA_1}}")
	if count != 2 {
		t.Errorf("el mismo valor debe reutilizar el marcador, encontrado %d veces en %q", count, clean)
	}
	if mapping["{{PERSONA_1}}"] != "María" {
		t.Errorf("mapping incorrecto: %v", mapping)
	}
}

func TestSanitizeNoPII(t *testing.T) {
	e := newTestEngine()
	clean, entities, _ := e.Sanitize("Reunión mañana a las 10")
	if clean != "Reunión mañana a las 10" {
		t.Errorf("texto sin PII alterado: %q", clean)
	}
	if len(entities) != 0 {
		t.Errorf("entidades inesperadas: %v", entities)
	}
}
