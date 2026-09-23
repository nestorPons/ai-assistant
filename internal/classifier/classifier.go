// Package classifier define la interfaz de clasificación y el contrato interno
// de decisión independiente del proveedor.
package classifier

import (
	"context"

	"github.com/nestorPons/ai-assistant/internal/domain"
)

// Decision es la ruta interna tras la decisión del orquestador.
type Decision string

const (
	DecisionDBAction Decision = "db_action"
	DecisionExtract  Decision = "extract"
	DecisionDiscard  Decision = "discard"
)

// Action describe una operación tipada sobre la base de datos.
type Action struct {
	Operation string         `json:"operation"`           // create | update | delete
	Entity    string         `json:"entity"`              // client | raw_message | task
	TargetID  string         `json:"target_id,omitempty"` // opcional
	Fields    map[string]any `json:"fields,omitempty"`
}

// ClassificationInput es la entrada de clasificación (contenido ya ofuscado).
type ClassificationInput struct {
	CleanPrompt    string
	Source         domain.Source
	ConversationID string
}

// ClassificationResult es el resultado interno de clasificación.
type ClassificationResult struct {
	Decision   Decision
	Confidence float64
	Reason     string
	// NeedsReview indica ambigüedad o revisión manual requerida.
	NeedsReview bool
	// Blocked indica que Jev bloqueó el procesamiento.
	Blocked bool
	// Action es obligatorio cuando Decision == DecisionDBAction.
	Action *Action
}

// Classifier es la interfaz independiente del proveedor.
type Classifier interface {
	Classify(ctx context.Context, input ClassificationInput) (ClassificationResult, error)
}
