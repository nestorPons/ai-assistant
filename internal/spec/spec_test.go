package spec

import (
	"strings"
	"testing"
	"time"

	"github.com/nestorPons/ai-assistant/internal/domain"
)

func TestRenderFullTask(t *testing.T) {
	due := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	task := &domain.Task{
		Title:        "Informe de ventas",
		Description:  "Preparar el informe del trimestre",
		Subject:      "ventas Q3",
		Priority:     domain.PriorityHigh,
		DueDate:      &due,
		Status:       domain.TaskStatusPending,
		AIConfidence: 0.95,
		Specifications: &domain.Specifications{
			Requirements:       []string{"Incluir datos de campaña"},
			AcceptanceCriteria: []string{"Entregable en PDF"},
			OpenQuestions:      []string{"¿Qué canal priorizar?"},
		},
	}

	md := Render(task)
	for _, want := range []string{
		"# Informe de ventas",
		"**Tema:** ventas Q3",
		"**Prioridad:** Alta",
		"**Fecha límite:** 2026-10-01",
		"**Estado:** Pendiente",
		"## Descripción",
		"Preparar el informe del trimestre",
		"## Requisitos",
		"- Incluir datos de campaña",
		"## Criterios de aceptación",
		"- Entregable en PDF",
		"## Preguntas abiertas",
		"- ¿Qué canal priorizar?",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("el spec.md no contiene %q\n---\n%s", want, md)
		}
	}
	if strings.Contains(md, "## Restricciones") {
		t.Error("no deben aparecer secciones vacías")
	}
}

func TestRenderMinimalTask(t *testing.T) {
	md := Render(&domain.Task{Title: "Arreglar la web"})
	for _, want := range []string{"# Arreglar la web", "**Fecha límite:** Sin fecha", "_Sin descripción._"} {
		if !strings.Contains(md, want) {
			t.Errorf("el spec.md no contiene %q", want)
		}
	}
}

func TestRenderNil(t *testing.T) {
	if got := Render(nil); got != "" {
		t.Errorf("Render(nil) = %q, want vacío", got)
	}
}
