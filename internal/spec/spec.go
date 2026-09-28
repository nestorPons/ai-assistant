// Package spec genera el documento spec.md de una tarea a partir de los datos
// ya extraídos. Es determinista y no depende de ningún LLM, de modo que el
// resultado es reproducible y editable después desde el dashboard.
package spec

import (
	"fmt"
	"strings"

	"github.com/nestorPons/ai-assistant/internal/domain"
)

var priorityLabels = map[domain.Priority]string{
	domain.PriorityLow:    "Baja",
	domain.PriorityMedium: "Media",
	domain.PriorityHigh:   "Alta",
}

var statusLabels = map[domain.TaskStatus]string{
	domain.TaskStatusPending:     "Pendiente",
	domain.TaskStatusInProgress:  "En progreso",
	domain.TaskStatusCompleted:   "Completada",
	domain.TaskStatusDiscarded:   "Descartada",
	domain.TaskStatusNeedsReview: "Revisión",
}

// Render devuelve el contenido Markdown (spec.md) de la tarea.
func Render(t *domain.Task) string {
	if t == nil {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", fallback(t.Title, "Tarea sin título"))
	fmt.Fprintf(&b, "- **Tema:** %s\n", fallback(t.Subject, "Sin tema"))
	fmt.Fprintf(&b, "- **Prioridad:** %s\n", label(priorityLabels, t.Priority, string(t.Priority)))
	if t.DueDate != nil {
		fmt.Fprintf(&b, "- **Fecha límite:** %s\n", t.DueDate.Format("2006-01-02"))
	} else {
		b.WriteString("- **Fecha límite:** Sin fecha\n")
	}
	fmt.Fprintf(&b, "- **Estado:** %s\n", label(statusLabels, t.Status, string(t.Status)))

	b.WriteString("## Descripción\n\n")
	b.WriteString(fallback(strings.TrimSpace(t.Description), "_Sin descripción._"))
	b.WriteString("\n")

	if s := t.Specifications; s != nil {
		writeList(&b, "Requisitos", s.Requirements)
		writeList(&b, "Restricciones", s.Constraints)
		writeList(&b, "Entregables", s.Deliverables)
		writeList(&b, "Criterios de aceptación", s.AcceptanceCriteria)
		writeList(&b, "Dependencias", s.Dependencies)
		writeList(&b, "Preguntas abiertas", s.OpenQuestions)
	}

	return b.String()
}

func writeList(b *strings.Builder, title string, items []string) {
	filled := make([]string, 0, len(items))
	for _, it := range items {
		if it = strings.TrimSpace(it); it != "" {
			filled = append(filled, it)
		}
	}
	if len(filled) == 0 {
		return
	}
	fmt.Fprintf(b, "\n## %s\n\n", title)
	for _, it := range filled {
		fmt.Fprintf(b, "- %s\n", it)
	}
}

func fallback(s, alt string) string {
	if strings.TrimSpace(s) == "" {
		return alt
	}
	return s
}

func label[K comparable](m map[K]string, k K, def string) string {
	if v, ok := m[k]; ok {
		return v
	}
	return def
}
