package extractor

import (
	"context"
	"testing"

	"github.com/nestorPons/ai-assistant/internal/domain"
	"github.com/nestorPons/ai-assistant/internal/llm"
)

type fakeProvider struct {
	content string
}

func (f fakeProvider) Complete(_ context.Context, _ llm.CompletionRequest) (llm.CompletionResponse, error) {
	return llm.CompletionResponse{Content: []byte(f.content)}, nil
}

func TestLLMExtractorParsesTask(t *testing.T) {
	content := `{
		"is_task": true,
		"confidence_score": 0.95,
		"task": {
			"title": "Informe de ventas",
			"description": "Preparar informe trimestral",
			"priority": "high",
			"estimated_hours": 2.5,
			"specifications": {
				"requirements": ["incluir datos"],
				"acceptance_criteria": ["entregable en PDF"]
			}
		}
	}`
	ext := NewLLMExtractor(fakeProvider{content: content}, "gpt-4o-mini")
	res, err := ext.Extract(context.Background(), ExtractionInput{CleanPrompt: "necesito informe"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsTask {
		t.Fatal("esperaba tarea")
	}
	if res.Title != "Informe de ventas" {
		t.Errorf("título = %q", res.Title)
	}
	if res.Priority != domain.PriorityHigh {
		t.Errorf("prioridad = %v", res.Priority)
	}
	if res.EstimatedHours == nil || *res.EstimatedHours != 2.5 {
		t.Errorf("horas = %v", res.EstimatedHours)
	}
	if res.Specifications == nil || len(res.Specifications.Requirements) != 1 {
		t.Errorf("especificaciones = %v", res.Specifications)
	}
}

func TestLLMExtractorNoTask(t *testing.T) {
	ext := NewLLMExtractor(fakeProvider{content: `{"is_task": false, "confidence_score": 0.9}`}, "gpt-4o-mini")
	res, err := ext.Extract(context.Background(), ExtractionInput{CleanPrompt: "gracias"})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsTask {
		t.Fatal("no esperaba tarea")
	}
}

func TestLLMExtractorInvalidJSON(t *testing.T) {
	ext := NewLLMExtractor(fakeProvider{content: `no json`}, "gpt-4o-mini")
	if _, err := ext.Extract(context.Background(), ExtractionInput{CleanPrompt: "x"}); err == nil {
		t.Fatal("esperaba error de respuesta inválida")
	}
}

func TestMockExtractor(t *testing.T) {
	ext := NewMockExtractor()
	res, err := ext.Extract(context.Background(), ExtractionInput{CleanPrompt: "urgente prepara el informe"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsTask {
		t.Fatal("esperaba tarea")
	}
	if res.Priority != domain.PriorityHigh {
		t.Errorf("prioridad = %v", res.Priority)
	}
}
