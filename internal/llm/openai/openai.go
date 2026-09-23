// Package openai implementa un adaptador LLMProvider para la API de OpenAI.
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/nestorPons/ai-assistant/internal/llm"
)

// Provider traduce CompletionRequest al protocolo de OpenAI.
type Provider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// New crea un adaptador OpenAI con base URL y clave.
func New(apiKey, baseURL string) *Provider {
	baseURL = strings.TrimRight(baseURL, "/")
	return &Provider{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	Temperature    float32         `json:"temperature,omitempty"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type       string           `json:"type"`
	JSONSchema *jsonSchemaValue `json:"json_schema,omitempty"`
}

type jsonSchemaValue struct {
	Name   string          `json:"name"`
	Strict bool            `json:"strict"`
	Schema json.RawMessage `json:"schema"`
}

type chatResponse struct {
	ID      string   `json:"id"`
	Model   string   `json:"model"`
	Choices []choice `json:"choices"`
	Usage   usage    `json:"usage"`
	Error   *apiErr  `json:"error,omitempty"`
}

type choice struct {
	Message      chatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type apiErr struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// Complete ejecuta una petición de completado con salida estructurada opcional.
func (p *Provider) Complete(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
	var resp llm.CompletionResponse

	msgs := []chatMessage{
		{Role: "system", Content: req.SystemPrompt},
		{Role: "user", Content: req.UserPrompt},
	}

	body := chatRequest{
		Model:       req.Model,
		Messages:    msgs,
		Temperature: req.Temperature,
		MaxTokens:   4096,
	}

	if len(req.JSONSchema) > 0 {
		var schema json.RawMessage
		if !json.Valid(req.JSONSchema) {
			return resp, llm.NewError(llm.ErrorInvalidResponse, errors.New("json schema inválido"))
		}
		schema = req.JSONSchema
		body.ResponseFormat = &responseFormat{
			Type: "json_schema",
			JSONSchema: &jsonSchemaValue{
				Name:   "structured_output",
				Strict: true,
				Schema: schema,
			},
		}
	} else {
		body.ResponseFormat = &responseFormat{Type: "json_object"}
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return resp, llm.NewError(llm.ErrorUnknown, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return resp, llm.NewError(llm.ErrorUnknown, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return resp, categorizeHTTPError(err)
	}
	defer httpResp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(httpResp.Body, 8<<20))
	if err != nil {
		return resp, llm.NewError(llm.ErrorUnknown, err)
	}

	if httpResp.StatusCode >= 400 {
		return resp, categorizeStatus(httpResp.StatusCode, data)
	}

	var cr chatResponse
	if err := json.Unmarshal(data, &cr); err != nil {
		return resp, llm.NewError(llm.ErrorInvalidResponse, fmt.Errorf("respuesta inválida: %w", err))
	}
	if cr.Error != nil {
		return resp, llm.NewError(categoryFromAPIError(cr.Error.Type, cr.Error.Code), errors.New(cr.Error.Message))
	}
	if len(cr.Choices) == 0 {
		return resp, llm.NewError(llm.ErrorInvalidResponse, errors.New("sin elecciones en la respuesta"))
	}

	content := cr.Choices[0].Message.Content
	if content == "" {
		return resp, llm.NewError(llm.ErrorInvalidResponse, errors.New("contenido vacío"))
	}

	resp = llm.CompletionResponse{
		Content:      []byte(content),
		Provider:     "openai",
		Model:        cr.Model,
		FinishReason: cr.Choices[0].FinishReason,
		Usage: llm.Usage{
			PromptTokens:     cr.Usage.PromptTokens,
			CompletionTokens: cr.Usage.CompletionTokens,
			TotalTokens:      cr.Usage.TotalTokens,
		},
	}
	return resp, nil
}

func categorizeHTTPError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return llm.NewError(llm.ErrorTimeout, err)
	}
	if ctxErr := err; ctxErr != nil && strings.Contains(err.Error(), "context") {
		return llm.NewError(llm.ErrorTimeout, err)
	}
	return llm.NewError(llm.ErrorUnavailable, err)
}

func categorizeStatus(status int, body []byte) error {
	var msg string
	var api apiErr
	if json.Unmarshal(body, &api) == nil && api.Message != "" {
		msg = api.Message
	} else {
		msg = strings.TrimSpace(string(body))
	}
	switch status {
	case http.StatusUnauthorized:
		return llm.NewError(llm.ErrorAuthentication, fmt.Errorf("status %d: %s", status, msg))
	case http.StatusTooManyRequests:
		return llm.NewError(llm.ErrorRateLimit, fmt.Errorf("status %d: %s", status, msg))
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		return llm.NewError(llm.ErrorTimeout, fmt.Errorf("status %d: %s", status, msg))
	case http.StatusBadGateway, http.StatusServiceUnavailable:
		return llm.NewError(llm.ErrorUnavailable, fmt.Errorf("status %d: %s", status, msg))
	default:
		return llm.NewError(llm.ErrorUnknown, fmt.Errorf("status %d: %s", status, msg))
	}
}

func categoryFromAPIError(t, code string) llm.ErrorCategory {
	switch {
	case t == "insufficient_quota" || code == "insufficient_quota" || strings.Contains(code, "auth"):
		return llm.ErrorAuthentication
	case t == "rate_limit_error" || code == "rate_limit_exceeded" || strings.Contains(code, "rate"):
		return llm.ErrorRateLimit
	case strings.Contains(code, "timeout"):
		return llm.ErrorTimeout
	default:
		return llm.ErrorUnknown
	}
}
