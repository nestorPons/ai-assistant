// Package jev implementa el adaptador JevAPIProvider contra el endpoint de
// decisiones de Jev. Traduce la respuesta externa a la ruta interna del sistema.
package jev

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

	"github.com/nestorPons/ai-assistant/internal/classifier"
	"github.com/nestorPons/ai-assistant/internal/llm"
)

// Provider llama a POST /api/v1/decisions/route de Jev con Bearer token.
type Provider struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// New crea un JevAPIProvider.
func New(baseURL, apiKey string, timeout time.Duration) *Provider {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &Provider{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		client:  &http.Client{Timeout: timeout},
	}
}

type routeRequest struct {
	Task        string          `json:"task"`
	Evidence    json.RawMessage `json:"evidence,omitempty"`
	Constraints json.RawMessage `json:"constraints,omitempty"`
}

type routeResponse struct {
	Code    int        `json:"code"`
	Message string     `json:"message"`
	Data    *routeData `json:"data"`
}

type routeData struct {
	Decision   string             `json:"decision"`
	Confidence float64            `json:"confidence"`
	Guidance   string             `json:"guidance"`
	Route      string             `json:"route,omitempty"`
	Action     *classifier.Action `json:"action,omitempty"`
}

// Classify ejecuta la decisión de Jev y la traduce a la ruta interna.
func (p *Provider) Classify(ctx context.Context, input classifier.ClassificationInput) (classifier.ClassificationResult, error) {
	var result classifier.ClassificationResult

	body := routeRequest{Task: input.CleanPrompt}

	raw, err := json.Marshal(body)
	if err != nil {
		return result, llm.NewError(llm.ErrorUnknown, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/v1/decisions/route", bytes.NewReader(raw))
	if err != nil {
		return result, llm.NewError(llm.ErrorUnknown, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return result, categorizeErr(err)
	}
	defer httpResp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(httpResp.Body, 1<<20))
	if err != nil {
		return result, llm.NewError(llm.ErrorUnknown, err)
	}

	if httpResp.StatusCode >= 400 {
		return result, categorizeStatus(httpResp.StatusCode, data)
	}

	var resp routeResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return result, llm.NewError(llm.ErrorInvalidResponse, fmt.Errorf("respuesta jev inválida: %w", err))
	}
	if resp.Code != 0 {
		return result, llm.NewError(llm.ErrorInvalidResponse, fmt.Errorf("jev code %d: %s", resp.Code, resp.Message))
	}
	if resp.Data == nil {
		return result, llm.NewError(llm.ErrorInvalidResponse, errors.New("jev sin data"))
	}

	return mapDecision(*resp.Data)
}

// mapDecision traduce la decisión de Jev a la ruta interna del sistema usando
// el vocabulario compartido de classifier.RouteDecision.
func mapDecision(d routeData) (classifier.ClassificationResult, error) {
	dec, needsReview, blocked, err := classifier.RouteDecision(d.Decision, d.Route)
	if err != nil {
		return classifier.ClassificationResult{}, llm.NewError(llm.ErrorInvalidResponse, err)
	}
	return classifier.ClassificationResult{
		Decision:    dec,
		Confidence:  d.Confidence,
		Reason:      d.Guidance,
		Action:      d.Action,
		NeedsReview: needsReview,
		Blocked:     blocked,
	}, nil
}

func categorizeErr(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return llm.NewError(llm.ErrorTimeout, err)
	}
	if strings.Contains(err.Error(), "context") {
		return llm.NewError(llm.ErrorTimeout, err)
	}
	return llm.NewError(llm.ErrorUnavailable, err)
}

// categorizeStatus normaliza el código HTTP de Jev a una categoría de error.
func categorizeStatus(status int, body []byte) error {
	msg := strings.TrimSpace(string(body))
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return llm.NewError(llm.ErrorAuthentication, fmt.Errorf("jev status %d: %s", status, msg))
	case http.StatusTooManyRequests:
		return llm.NewError(llm.ErrorRateLimit, fmt.Errorf("jev status %d: %s", status, msg))
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		return llm.NewError(llm.ErrorTimeout, fmt.Errorf("jev status %d: %s", status, msg))
	case http.StatusBadGateway, http.StatusServiceUnavailable:
		return llm.NewError(llm.ErrorUnavailable, fmt.Errorf("jev status %d: %s", status, msg))
	default:
		return llm.NewError(llm.ErrorUnknown, fmt.Errorf("jev status %d: %s", status, msg))
	}
}
