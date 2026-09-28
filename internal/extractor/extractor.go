// Package extractor define la interfaz de extracción estructurada y el contrato
// de salida del extractor de frontera.
package extractor

import (
	"context"
	"time"

	"github.com/nestorPons/ai-assistant/internal/domain"
)

// ExtractionInput es la entrada del extractor (contenido ya ofuscado).
type ExtractionInput struct {
	CleanPrompt string
	Source      domain.Source
	// Reference es la fecha/hora de referencia para resolver fechas relativas
	// ("mañana", "el lunes", "en 3 días"). Si es cero se usa la hora actual.
	Reference time.Time
}

// ExtractionResult es la salida estructurada validada del extractor.
type ExtractionResult struct {
	IsTask         bool
	Confidence     float64
	Title          string
	Description    string
	Subject        string
	Priority       domain.Priority
	DueDate        *time.Time
	Specifications *domain.Specifications
}

// Extractor es la interfaz independiente del proveedor.
type Extractor interface {
	Extract(ctx context.Context, input ExtractionInput) (ExtractionResult, error)
}

// extractionJSONSchema define el JSON Schema de Structured Outputs.
// En modo strict, OpenAI exige que todo objeto declare additionalProperties=false
// y que required incluya todas sus propiedades; los campos opcionales se aceptan
// como null. La descripción resume la tarea y specifications conserva sus
// requisitos operativos.
const extractionJSONSchema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["is_task", "confidence_score", "task"],
  "properties": {
    "is_task": {"type": "boolean"},
    "confidence_score": {"type": "number"},
    "task": {
      "type": ["object", "null"],
      "additionalProperties": false,
      "required": ["title", "description", "subject", "priority", "due_date", "specifications"],
      "properties": {
        "title": {"type": "string"},
        "description": {
          "type": "string",
          "description": "Qué hay que hacer, estimado de forma concisa y accionable: 2-3 frases como máximo, sin copiar el mensaje literal, sin repetir el título o el tema y sin relleno."
        },
        "subject": {
          "type": "string",
          "description": "Tema del trabajo: proyecto, web, cliente, sistema o asunto sobre el que se trabaja. Obligatorio si is_task=true."
        },
        "priority": {"type": "string", "enum": ["low", "medium", "high"]},
        "due_date": {
          "type": ["string", "null"],
          "description": "Fecha límite en formato YYYY-MM-DD, o null si el mensaje no la indica."
        },
        "specifications": {
          "type": ["object", "null"],
          "additionalProperties": false,
          "required": ["requirements", "constraints", "deliverables", "acceptance_criteria", "dependencies", "open_questions"],
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
