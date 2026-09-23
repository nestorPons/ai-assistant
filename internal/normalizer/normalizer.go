// Package normalizer transforma y valida eventos heterogéneos en IncomingMessage.
package normalizer

import (
	"context"
	"errors"
	"strings"

	"github.com/nestorPons/ai-assistant/internal/domain"
)

// Normalizer normaliza un IncomingMessage (recorte, identificador, canal).
type Normalizer struct{}

// New crea un Normalizer.
func New() *Normalizer { return &Normalizer{} }

// Normalize valida y normaliza el mensaje en el sitio.
func (n *Normalizer) Normalize(_ context.Context, msg *domain.IncomingMessage) error {
	if msg == nil {
		return errors.New("mensaje nulo")
	}
	if !msg.Source.Valid() {
		return errors.New("canal no soportado")
	}

	msg.ClientIdentifier = normalizeIdentifier(msg.Source, msg.ClientIdentifier)
	if msg.ClientIdentifier == "" {
		return errors.New("identificador de cliente vacío")
	}
	msg.ClientName = strings.TrimSpace(msg.ClientName)
	msg.RawContent = strings.TrimSpace(msg.RawContent)
	if msg.RawContent == "" {
		return errors.New("contenido vacío")
	}
	if msg.ID == "" {
		return errors.New("id externo vacío")
	}
	return nil
}

// normalizeIdentifier normaliza el identificador según el canal.
func normalizeIdentifier(source domain.Source, id string) string {
	id = strings.TrimSpace(id)
	switch source {
	case domain.SourceGmail:
		return strings.ToLower(id)
	default:
		return id
	}
}

// WordCount cuenta las palabras de un texto (para el pre-filtro ligero).
func WordCount(s string) int {
	return len(strings.Fields(s))
}
