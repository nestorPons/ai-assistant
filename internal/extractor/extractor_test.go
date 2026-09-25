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
			"subject": "ventas Q3",
			"priority": "high",
			"estimated_hours": 2.5,
			"due_date": "2026-10-01",
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
	if res.Subject != "ventas Q3" {
		t.Errorf("tema = %q", res.Subject)
	}
	if res.Priority != domain.PriorityHigh {
		t.Errorf("prioridad = %v", res.Priority)
	}
	if res.EstimatedHours == nil || *res.EstimatedHours != 2.5 {
		t.Errorf("horas = %v", res.EstimatedHours)
	}
	if res.DueDate == nil || res.DueDate.Format("2006-01-02") != "2026-10-01" {
		t.Errorf("fecha límite = %v", res.DueDate)
	}
	if res.Specifications == nil || len(res.Specifications.Requirements) != 1 {
		t.Errorf("especificaciones = %v", res.Specifications)
	}
}

func TestLLMExtractorTaskWithoutDateOrParams(t *testing.T) {
	content := `{
		"is_task": true,
		"confidence_score": 0.9,
		"task": {
			"title": "Arreglar lo que no funciona",
			"description": "Revisar y arreglar la web",
			"subject": "nestorpons.com",
			"priority": "high",
			"estimated_hours": null,
			"due_date": null,
			"specifications": {}
		}
	}`
	ext := NewLLMExtractor(fakeProvider{content: content}, "gpt-4o-mini")
	res, err := ext.Extract(context.Background(), ExtractionInput{CleanPrompt: "arreglame la web"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsTask {
		t.Fatal("una petición sin fecha debe seguir siendo tarea")
	}
	if res.Subject != "nestorpons.com" {
		t.Errorf("tema = %q", res.Subject)
	}
	if res.DueDate != nil {
		t.Errorf("fecha límite debería ser nil, fue %v", res.DueDate)
	}
}

func TestLLMExtractorSubjectFallsBackToTitle(t *testing.T) {
	content := `{
		"is_task": true,
		"confidence_score": 0.9,
		"task": {
			"title": "Revisar factura",
			"description": "Revisar la factura pendiente",
			"subject": "",
			"priority": "medium"
		}
	}`
	ext := NewLLMExtractor(fakeProvider{content: content}, "gpt-4o-mini")
	res, err := ext.Extract(context.Background(), ExtractionInput{CleanPrompt: "revisa la factura"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Subject != "Revisar factura" {
		t.Errorf("tema de respaldo = %q", res.Subject)
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

func TestMockExtractorGuessSubject(t *testing.T) {
	ext := NewMockExtractor()
	res, err := ext.Extract(context.Background(), ExtractionInput{CleanPrompt: "Arreglame lo que no funciona en nestorpons.com lo antes posible"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Subject != "nestorpons.com" {
		t.Errorf("tema = %q", res.Subject)
	}
	if res.DueDate != nil {
		t.Errorf("fecha límite debería ser nil, fue %v", res.DueDate)
	}
}
