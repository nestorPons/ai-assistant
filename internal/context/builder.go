// Package context construye el sobre de análisis con metadatos no sensibles y el
// mensaje ofuscado delimitado. Nunca mezcla instrucciones del sistema con el
// texto del usuario ni incluye datos personales originales.
package context

import (
	"encoding/json"
	"time"

	"github.com/nestorPons/ai-assistant/internal/domain"
)

// Envelope es el contexto controlado enviado a Jev/extractor.
type Envelope struct {
	Source     string    `json:"source"`
	ReceivedAt time.Time `json:"received_at"`
	Message    string    `json:"message"`
}

// Builder construye el sobre de análisis.
type Builder struct{}

// New crea un ContextBuilder.
func New() *Builder { return &Builder{} }

// Build genera el sobre JSON con metadatos no sensibles y el mensaje delimitado.
// cleanPrompt ya debe estar ofuscado por el anonimizador.
func (b *Builder) Build(msg domain.IncomingMessage, cleanPrompt string) (string, error) {
	env := Envelope{
		Source:     string(msg.Source),
		ReceivedAt: msg.ReceivedAt.UTC(),
		Message:    "Contenido delimitado a analizar:\n<<<BEGIN_MESSAGE>>>\n" + cleanPrompt + "\n<<<END_MESSAGE>>>",
	}
	raw, err := json.Marshal(env)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
