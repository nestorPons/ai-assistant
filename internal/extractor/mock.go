package extractor

import (
	"context"
	"strings"

	"github.com/nestorPons/ai-assistant/internal/domain"
)

// MockExtractor genera una tarea básica a partir del mensaje para desarrollo.
type MockExtractor struct{}

// NewMockExtractor crea un extractor de prueba.
func NewMockExtractor() *MockExtractor { return &MockExtractor{} }

// Extract genera una tarea simple con prioridad según urgencia.
func (m *MockExtractor) Extract(_ context.Context, input ExtractionInput) (ExtractionResult, error) {
	lower := strings.ToLower(input.CleanPrompt)

	if !looksLikeTask(lower) {
		return ExtractionResult{IsTask: false, Confidence: 0.95}, nil
	}

	priority := domain.PriorityMedium
	switch {
	case strings.Contains(lower, "urgente") || strings.Contains(lower, "para hoy") || strings.Contains(lower, "ya"):
		priority = domain.PriorityHigh
	case strings.Contains(lower, "cuando puedas") || strings.Contains(lower, "sin prisa"):
		priority = domain.PriorityLow
	}

	specs := &domain.Specifications{
		Requirements: []string{"Requisito extraído del mensaje"},
	}

	title := firstSentence(input.CleanPrompt)
	return ExtractionResult{
		IsTask:         true,
		Confidence:     0.92,
		Title:          title,
		Description:    strings.TrimSpace(input.CleanPrompt),
		Subject:        guessSubject(input.CleanPrompt, title),
		Priority:       priority,
		Specifications: specs,
	}, nil
}

// guessSubject aproxima el tema del trabajo (proyecto, web, dominio…) a partir
// del primer token que parece un dominio o URL; si no lo encuentra usa el título.
func guessSubject(s, fallback string) string {
	for _, field := range strings.Fields(s) {
		f := strings.Trim(field, ".,;:!?¡¿()[]{}\"'")
		if strings.HasPrefix(f, "http://") || strings.HasPrefix(f, "https://") {
			return strings.TrimPrefix(strings.TrimPrefix(f, "https://"), "http://")
		}
		if strings.Contains(f, ".") && !strings.HasSuffix(f, ".") {
			return f
		}
	}
	return fallback
}

func looksLikeTask(s string) bool {
	keywords := []string{"necesito", "prepara", "haz", "realiza", "encargo", "tarea", "informe", "revisa", "preparar", "hacer", "arregla", "soluciona", "corrige", "fix"}
	for _, k := range keywords {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

func firstSentence(s string) string {
	title := strings.TrimSpace(s)
	if i := strings.IndexAny(title, ".!\n"); i > 0 {
		title = title[:i]
	}
	const max = 140
	runes := []rune(title)
	if len(runes) > max {
		title = string(runes[:max])
	}
	return strings.TrimSpace(title)
}
