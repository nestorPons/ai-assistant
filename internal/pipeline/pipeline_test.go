package pipeline

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nestorPons/ai-assistant/internal/anonymizer"
	"github.com/nestorPons/ai-assistant/internal/anonymizer/detect"
	"github.com/nestorPons/ai-assistant/internal/classifier"
	"github.com/nestorPons/ai-assistant/internal/domain"
	"github.com/nestorPons/ai-assistant/internal/extractor"
	"github.com/nestorPons/ai-assistant/internal/persistence"
)

func newTestPipeline(t *testing.T) (*Pipeline, *persistence.MemoryStore) {
	t.Helper()

	// Anonimizador real en proceso.
	detector := detect.New(nil, nil, nil)
	engine := anonymizer.NewEngine(detector, detect.NoopNER{})
	store := anonymizer.NewMemoryMappingStore()
	anonSrv := anonymizer.NewServer(engine, store, anonymizer.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(anonSrv.Handler())
	t.Cleanup(ts.Close)

	client := anonymizer.NewClient(ts.URL, "", 5*time.Second)

	mem := persistence.NewMemoryStore()
	p := New(
		mem,
		client,
		classifier.NewMockRuleClassifier(),
		extractor.NewMockExtractor(),
		Config{MinWords: 3, ExtractConfidence: 0.6, ReviewBelowConfidence: 0.5},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	return p, mem
}

func seedClient(t *testing.T, mem *persistence.MemoryStore, source domain.Source, identifier string, tracked, active bool) *domain.Client {
	t.Helper()
	c := &domain.Client{Source: source, Identifier: identifier, Name: "Cliente", Tracked: tracked, Active: active}
	if err := mem.CreateClient(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	return c
}

func msg(id, source, identifier, content string) domain.IncomingMessage {
	return domain.IncomingMessage{
		ID:               id,
		Source:           domain.Source(source),
		ClientIdentifier: identifier,
		ClientName:       "Cliente",
		RawContent:       content,
		ReceivedAt:       time.Now().UTC(),
	}
}

func TestPipelineCreatesTask(t *testing.T) {
	p, mem := newTestPipeline(t)
	seedClient(t, mem, domain.SourceGmail, "maria@example.com", true, true)

	err := p.Process(context.Background(), msg("m1", "gmail", "maria@example.com", "Necesito que prepares el informe de ventas urgente"))
	if err != nil {
		t.Fatal(err)
	}

	tasks, err := mem.ListTasks(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("esperaba 1 tarea, hay %d", len(tasks))
	}
	if tasks[0].Status != domain.TaskStatusPending {
		t.Errorf("estado = %v", tasks[0].Status)
	}
	if tasks[0].Subject == "" {
		t.Error("la tarea debe tener un tema (subject)")
	}
}

func TestPipelineExcludesUnauthorized(t *testing.T) {
	p, mem := newTestPipeline(t)
	seedClient(t, mem, domain.SourceGmail, "desconocido@example.com", false, true)

	err := p.Process(context.Background(), msg("m1", "gmail", "desconocido@example.com", "Necesito que prepares el informe urgente"))
	if err != nil {
		t.Fatal(err)
	}

	tasks, _ := mem.ListTasks(context.Background(), nil)
	if len(tasks) != 0 {
		t.Fatalf("no debe crear tareas de usuarios no autorizados")
	}
	msgs, _ := mem.ListRawMessages(context.Background(), "")
	if len(msgs) != 0 {
		t.Fatalf("no debe guardar mensajes de usuarios no autorizados")
	}
}

func TestPipelineIdempotent(t *testing.T) {
	p, mem := newTestPipeline(t)
	seedClient(t, mem, domain.SourceGmail, "maria@example.com", true, true)

	m := msg("dup", "gmail", "maria@example.com", "Necesito que prepares el informe urgente")
	if err := p.Process(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	if err := p.Process(context.Background(), m); err != nil {
		t.Fatal(err)
	}

	tasks, _ := mem.ListTasks(context.Background(), nil)
	if len(tasks) != 1 {
		t.Fatalf("mensaje duplicado procesado dos veces: %d tareas", len(tasks))
	}
}

func TestPipelineDiscardsCortesy(t *testing.T) {
	p, mem := newTestPipeline(t)
	seedClient(t, mem, domain.SourceGmail, "maria@example.com", true, true)

	if err := p.Process(context.Background(), msg("m1", "gmail", "maria@example.com", "Gracias, perfecto")); err != nil {
		t.Fatal(err)
	}

	tasks, _ := mem.ListTasks(context.Background(), nil)
	if len(tasks) != 0 {
		t.Fatalf("cortesía no debe generar tarea")
	}
}

func TestPipelinePrefilterShortMessage(t *testing.T) {
	p, mem := newTestPipeline(t)
	seedClient(t, mem, domain.SourceGmail, "maria@example.com", true, true)

	if err := p.Process(context.Background(), msg("m1", "gmail", "maria@example.com", "ok")); err != nil {
		t.Fatal(err)
	}

	tasks, _ := mem.ListTasks(context.Background(), nil)
	if len(tasks) != 0 {
		t.Fatalf("mensaje corto no debe generar tarea")
	}
}
