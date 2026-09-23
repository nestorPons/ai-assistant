package normalizer

import (
	"context"
	"testing"

	"github.com/nestorPons/ai-assistant/internal/domain"
)

func TestNormalizeGmailLowercasesIdentifier(t *testing.T) {
	n := New()
	msg := &domain.IncomingMessage{
		ID:               "id1",
		Source:           domain.SourceGmail,
		ClientIdentifier: "  Maria@Example.COM ",
		RawContent:       "  hola  ",
	}
	if err := n.Normalize(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	if msg.ClientIdentifier != "maria@example.com" {
		t.Errorf("identificador no normalizado: %q", msg.ClientIdentifier)
	}
	if msg.RawContent != "hola" {
		t.Errorf("contenido no recortado: %q", msg.RawContent)
	}
}

func TestNormalizeRejectsInvalid(t *testing.T) {
	n := New()
	cases := []domain.IncomingMessage{
		{Source: "invalid"},
		{Source: domain.SourceGmail, ClientIdentifier: ""},
		{Source: domain.SourceGmail, ClientIdentifier: "a@b.com", RawContent: "  "},
		{Source: domain.SourceGmail, ClientIdentifier: "a@b.com", RawContent: "x", ID: ""},
	}
	for i, c := range cases {
		if err := n.Normalize(context.Background(), &c); err == nil {
			t.Errorf("caso %d debería fallar", i)
		}
	}
}

func TestWordCount(t *testing.T) {
	if WordCount("uno dos tres") != 3 {
		t.Error("word count incorrecto")
	}
	if WordCount("ok") != 1 {
		t.Error("word count incorrecto")
	}
}
