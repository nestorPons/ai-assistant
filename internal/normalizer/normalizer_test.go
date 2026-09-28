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

func TestNormalizeWhatsAppCanonical(t *testing.T) {
	n := New()
	cases := map[string]string{
		" +34 612 345 678 ":  "+34612345678",
		"612345678":          "+612345678",
		"123456789-123@g.us": "123456789-123@g.us",
	}
	for in, want := range cases {
		msg := &domain.IncomingMessage{
			ID:               "id",
			Source:           domain.SourceWhatsApp,
			ClientIdentifier: in,
			RawContent:       "hola",
		}
		if err := n.Normalize(context.Background(), msg); err != nil {
			t.Fatalf("normalize(%q): %v", in, err)
		}
		if msg.ClientIdentifier != want {
			t.Errorf("identificador %q = %q, want %q", in, msg.ClientIdentifier, want)
		}
	}
}
