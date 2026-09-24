// Package classifier define la interfaz de clasificación y el contrato interno
// de decisión independiente del proveedor.
package classifier

import (
	"context"
	"fmt"
	"strings"

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

// RouteDecision traduce la decisión de un orquestador externo al vocabulario
// interno. Acepta tanto el vocabulario de gating
// (proceed_fast|deep_review|split_task|block) como el interno
// (db_action|extract|discard). proceed_fast resuelve la ruta vía route
// (por defecto extract).
func RouteDecision(decision, route string) (dec Decision, needsReview, blocked bool, err error) {
	switch strings.ToLower(strings.TrimSpace(decision)) {
	case string(DecisionDBAction), string(DecisionExtract), string(DecisionDiscard):
		return Decision(strings.ToLower(strings.TrimSpace(decision))), false, false, nil

	case "proceed_fast":
		switch strings.ToLower(strings.TrimSpace(route)) {
		case "", "extract":
			return DecisionExtract, false, false, nil
		case "db_action":
			return DecisionDBAction, false, false, nil
		case "discard":
			return DecisionDiscard, false, false, nil
		default:
			return DecisionExtract, false, false, nil
		}

	case "deep_review", "split_task":
		return DecisionDiscard, true, false, nil

	case "block":
		return DecisionDiscard, false, true, nil

	default:
		return "", false, false, fmt.Errorf("decisión desconocida: %q", decision)
	}
}
