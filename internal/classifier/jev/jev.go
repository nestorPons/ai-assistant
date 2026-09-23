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
		return result, llm.NewError(llm.ErrorUnknown, fmt.Errorf("jev status %d: %s", httpResp.StatusCode, strings.TrimSpace(string(data))))
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

// mapDecision traduce la decisión de Jev a la ruta interna del sistema.
// Acepta tanto el vocabulario de gating (proceed_fast|deep_review|split_task|block)
// como el vocabulario interno (db_action|extract|discard).
func mapDecision(d routeData) (classifier.ClassificationResult, error) {
	r := classifier.ClassificationResult{
		Confidence: d.Confidence,
		Reason:     d.Guidance,
		Action:     d.Action,
	}

	decision := strings.ToLower(strings.TrimSpace(d.Decision))
	switch decision {
	case string(classifier.DecisionDBAction), string(classifier.DecisionExtract), string(classifier.DecisionDiscard):
		r.Decision = classifier.Decision(decision)
		return r, nil

	case "proceed_fast":
		switch strings.ToLower(strings.TrimSpace(d.Route)) {
		case "", "extract":
			r.Decision = classifier.DecisionExtract
		case "db_action":
			r.Decision = classifier.DecisionDBAction
		case "discard":
			r.Decision = classifier.DecisionDiscard
		default:
			r.Decision = classifier.DecisionExtract
		}
		return r, nil

	case "deep_review", "split_task":
		r.NeedsReview = true
		r.Decision = classifier.DecisionDiscard
		return r, nil

	case "block":
		r.Blocked = true
		r.Decision = classifier.DecisionDiscard
		return r, nil

	default:
		return r, llm.NewError(llm.ErrorInvalidResponse, fmt.Errorf("decisión desconocida: %q", d.Decision))
	}
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
