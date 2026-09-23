package context

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/nestorPons/ai-assistant/internal/domain"
)

func TestBuilderDelimitsMessage(t *testing.T) {
	b := New()
	msg := domain.IncomingMessage{
		Source:     domain.SourceGmail,
		ReceivedAt: time.Now().UTC(),
	}
	raw, err := b.Build(msg, "contenido ofuscado {{PERSONA_1}}")
	if err != nil {
		t.Fatal(err)
	}

	var env Envelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		t.Fatalf("contexto no es JSON válido: %v", err)
	}
	if env.Source != "gmail" {
		t.Errorf("source = %q", env.Source)
	}
	if !strings.Contains(env.Message, "<<<BEGIN_MESSAGE>>>") || !strings.Contains(env.Message, "<<<END_MESSAGE>>>") {
		t.Errorf("mensaje no delimitado: %q", env.Message)
	}
	if !strings.Contains(env.Message, "{{PERSONA_1}}") {
		t.Errorf("falta contenido ofuscado: %q", env.Message)
	}
}
