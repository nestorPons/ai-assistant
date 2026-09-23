// Package simulated proporciona una fuente de mensajes de prueba para validar el
// pipeline sin depender de un canal real.
package simulated

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/nestorPons/ai-assistant/internal/domain"
)

// Source emite una secuencia fija de mensajes y luego espera.
type Source struct {
	messages []domain.IncomingMessage
	interval time.Duration
}

// New crea una fuente con los mensajes indicados.
func New(messages []domain.IncomingMessage, interval time.Duration) *Source {
	if interval <= 0 {
		interval = time.Second
	}
	return &Source{messages: messages, interval: interval}
}

// NewDefault genera mensajes de demostración.
func NewDefault() *Source {
	now := time.Now().UTC()
	return New([]domain.IncomingMessage{
		{
			ID:               uuid.NewString(),
			Source:           domain.SourceGmail,
			ClientIdentifier: "maria@example.com",
			ClientName:       "María García",
			RawContent:       "Hola, necesito que prepares el informe de ventas del trimestre. Es urgente, para hoy. Adjunta los datos de la campaña.",
			ReceivedAt:       now,
		},
		{
			ID:               uuid.NewString(),
			Source:           domain.SourceGmail,
			ClientIdentifier: "maria@example.com",
			ClientName:       "María García",
			RawContent:       "Gracias, perfecto.",
			ReceivedAt:       now.Add(time.Second),
		},
	}, time.Second)
}

// Name identifica el canal.
func (s *Source) Name() string { return "simulated" }

// Start emite los mensajes y espera la cancelación del contexto.
func (s *Source) Start(ctx context.Context, out chan<- domain.IncomingMessage) error {
	for i := range s.messages {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case out <- s.messages[i]:
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(s.interval):
		}
	}

	<-ctx.Done()
	return ctx.Err()
}
