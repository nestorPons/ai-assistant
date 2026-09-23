// Package anonymizer define los contratos del microservicio llm-anonymizer y el
// cliente HTTP usado por core-engine. El contenido original se procesa solo en
// memoria; la tabla de correspondencias nunca se devuelve al cliente.
package anonymizer

import "github.com/nestorPons/ai-assistant/internal/anonymizer/detect"

// SanitizeRequest es la petición de ofuscación.
type SanitizeRequest struct {
	Prompt         string `json:"prompt"`
	ConversationID string `json:"conversation_id,omitempty"`
	Mode           string `json:"mode,omitempty"`
}

// EntityMatch describe una entidad sustituida (sin el valor original).
type EntityMatch struct {
	Type  detect.EntityType `json:"type"`
	Token string            `json:"token"`
	Start int               `json:"start"`
	End   int               `json:"end"`
}

// SanitizeResponse es la respuesta de ofuscación.
type SanitizeResponse struct {
	RequestID   string        `json:"request_id"`
	CleanPrompt string        `json:"clean_prompt"`
	Entities    []EntityMatch `json:"entities,omitempty"`
}

// RevertRequest solicita la restauración de tokens en un texto procesado.
type RevertRequest struct {
	RequestID     string `json:"request_id"`
	ProcessedText string `json:"processed_text"`
}

// RevertResponse devuelve el texto con los valores reales restaurados.
type RevertResponse struct {
	RequestID    string `json:"request_id"`
	RevertedText string `json:"reverted_text"`
}

// ErrorResponse es el cuerpo de error estable y no sensible.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody contiene código y mensaje públicos genéricos.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
