// Package extractor define la interfaz de extracción estructurada y el contrato
// de salida del extractor de frontera.
package extractor

import (
	"context"

	"github.com/nestorPons/ai-assistant/internal/domain"
)

// ExtractionInput es la entrada del extractor (contenido ya ofuscado).
type ExtractionInput struct {
	CleanPrompt string
	Source      domain.Source
}

// ExtractionResult es la salida estructurada validada del extractor.
type ExtractionResult struct {
	IsTask         bool
	Confidence     float64
	Title          string
	Description    string
	Priority       domain.Priority
	EstimatedHours *float64
	Specifications *domain.Specifications
}

// Extractor es la interfaz independiente del proveedor.
type Extractor interface {
	Extract(ctx context.Context, input ExtractionInput) (ExtractionResult, error)
}

// extractionJSONSchema define el JSON Schema de Structured Outputs.
// La descripción resume la tarea; specifications conserva sus requisitos operativos.
const extractionJSONSchema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["is_task", "confidence_score"],
  "properties": {
    "is_task": {"type": "boolean"},
    "confidence_score": {"type": "number"},
    "task": {
      "type": "object",
      "additionalProperties": false,
      "required": ["title", "description", "priority"],
      "properties": {
        "title": {"type": "string"},
        "description": {"type": "string"},
        "priority": {"type": "string", "enum": ["low", "medium", "high"]},
        "estimated_hours": {"type": "number"},
        "specifications": {
          "type": "object",
          "additionalProperties": false,
          "properties": {
            "requirements": {"type": "array", "items": {"type": "string"}},
            "constraints": {"type": "array", "items": {"type": "string"}},
            "deliverables": {"type": "array", "items": {"type": "string"}},
            "acceptance_criteria": {"type": "array", "items": {"type": "string"}},
            "dependencies": {"type": "array", "items": {"type": "string"}},
            "open_questions": {"type": "array", "items": {"type": "string"}}
          }
        }
      }
    }
  }
}`
