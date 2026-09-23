// Package worker ejecuta el procesamiento concurrente de mensajes con reintentos.
package worker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/nestorPons/ai-assistant/internal/domain"
	"github.com/nestorPons/ai-assistant/internal/pipeline"
)

// Worker consume mensajes de un canal y los procesa en goroutines.
type Worker struct {
	pipeline    *pipeline.Pipeline
	logger      *slog.Logger
	concurrency int
	maxRetries  int
}

// New crea un Worker.
func New(p *pipeline.Pipeline, concurrency, maxRetries int, logger *slog.Logger) *Worker {
	if concurrency <= 0 {
		concurrency = 4
	}
	if maxRetries < 0 {
		maxRetries = 2
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Worker{pipeline: p, logger: logger, concurrency: concurrency, maxRetries: maxRetries}
}

// Run procesa mensajes hasta que el contexto se cancela o el canal se cierra.
func (w *Worker) Run(ctx context.Context, input <-chan domain.IncomingMessage) {
	var wg sync.WaitGroup
	for i := 0; i < w.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case msg, ok := <-input:
					if !ok {
						return
					}
					w.process(ctx, msg)
				}
			}
		}()
	}
	wg.Wait()
}

func (w *Worker) process(ctx context.Context, msg domain.IncomingMessage) {
	var err error
	for attempt := 0; attempt <= w.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<(attempt-1)) * time.Second
			w.logger.Warn("reintentando mensaje", "id", msg.ID, "attempt", attempt, "backoff", backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
		}

		err = w.pipeline.Process(ctx, msg)
		if err == nil {
			return
		}
		w.logger.Error("error procesando mensaje", "id", msg.ID, "attempt", attempt, "error", err)
	}
	w.logger.Error("mensaje agotado tras reintentos", "id", msg.ID, "attempts", w.maxRetries)
}
