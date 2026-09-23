// Package ingestion define los adaptadores de entrada (canales) que producen
// mensajes normalizados hacia el pipeline.
package ingestion

import (
	"context"

	"github.com/nestorPons/ai-assistant/internal/domain"
)

// Source es un canal de entrada que emite IncomingMessage por el canal de salida.
type Source interface {
	// Name identifica el canal para logs.
	Name() string
	// Start comienza a escuchar y bloquea hasta que el contexto se cancela.
	Start(ctx context.Context, out chan<- domain.IncomingMessage) error
}
