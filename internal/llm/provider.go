// Package llm define la abstracción interna de proveedores LLM.
// El dominio, el clasificador y el extractor solo dependen de estas interfaces;
// los SDK externos se usan únicamente dentro de adaptadores concretos.
package llm

import (
	"context"
	"fmt"
)

// ErrorCategory normaliza errores externos a categorías internas para que los
// reintentos y la revisión no dependan del proveedor concreto.
type ErrorCategory string

const (
	ErrorAuthentication  ErrorCategory = "authentication"
	ErrorRateLimit       ErrorCategory = "rate_limit"
	ErrorTimeout         ErrorCategory = "timeout"
	ErrorUnavailable     ErrorCategory = "unavailable"
	ErrorInvalidResponse ErrorCategory = "invalid_response"
	ErrorUnknown         ErrorCategory = "unknown"
)

// Error representa un error de proveedor con categoría normalizada.
type Error struct {
	Category ErrorCategory
	Err      error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Category, e.Err)
	}
	return string(e.Category)
}

func (e *Error) Unwrap() error { return e.Err }

// Retryable indica si el error admite reintentos temporales.
func (e *Error) Retryable() bool {
	switch e.Category {
	case ErrorRateLimit, ErrorTimeout, ErrorUnavailable:
		return true
	default:
		return false
	}
}

// NewError construye un error normalizado.
func NewError(cat ErrorCategory, err error) *Error {
	return &Error{Category: cat, Err: err}
}

// Usage describe el consumo devuelto por un proveedor.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// CompletionRequest es la petición interna hacia un proveedor.
type CompletionRequest struct {
	Model        string
	SystemPrompt string
	UserPrompt   string
	JSONSchema   []byte
	Temperature  float32
}

// CompletionResponse es la respuesta interna normalizada.
type CompletionResponse struct {
	Content      []byte
	Provider     string
	Model        string
	FinishReason string
	Usage        Usage
}

// LLMProvider es la interfaz estable hacia cualquier proveedor LLM.
type LLMProvider interface {
	Complete(ctx context.Context, request CompletionRequest) (CompletionResponse, error)
}
