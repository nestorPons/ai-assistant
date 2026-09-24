package classifier

import (
	"context"
	"log/slog"
)

// Chain intenta cada clasificador en orden y devuelve el primero que responde.
// Si todas las etapas fallan, marca el mensaje para revisión en vez de perderlo.
type Chain struct {
	stages []Classifier
	logger *slog.Logger
}

// NewChain crea una cadena de fallback con las etapas en orden de prioridad.
func NewChain(logger *slog.Logger, stages ...Classifier) *Chain {
	if logger == nil {
		logger = slog.Default()
	}
	return &Chain{stages: stages, logger: logger}
}

// Classify recorre las etapas; ante error de una, pasa a la siguiente.
func (c *Chain) Classify(ctx context.Context, input ClassificationInput) (ClassificationResult, error) {
	for i, stage := range c.stages {
		if stage == nil {
			continue
		}

		result, err := stage.Classify(ctx, input)
		if err == nil {
			if i > 0 {
				c.logger.Info("clasificador de fallback usado", "stage", i, "decision", result.Decision)
			}
			return result, nil
		}
		if ctx.Err() != nil {
			return ClassificationResult{}, ctx.Err()
		}
		c.logger.Warn("clasificador falló, probando siguiente", "stage", i, "error", err)
	}

	c.logger.Error("todos los clasificadores fallaron, enviando a revisión")
	return ClassificationResult{
		Decision:    DecisionDiscard,
		NeedsReview: true,
		Reason:      "todos los clasificadores fallaron",
	}, nil
}
