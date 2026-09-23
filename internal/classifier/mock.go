package classifier

import (
	"context"
	"strings"
)

// ClassifierFunc adapta una función a la interfaz Classifier.
type ClassifierFunc func(ctx context.Context, input ClassificationInput) (ClassificationResult, error)

// Classify implementa Classifier.
func (f ClassifierFunc) Classify(ctx context.Context, input ClassificationInput) (ClassificationResult, error) {
	return f(ctx, input)
}

// MockRuleClassifier es un clasificador heurístico para desarrollo y pruebas.
// No depende de Jev: enruta por palabras clave.
type MockRuleClassifier struct{}

// NewMockRuleClassifier crea un clasificador de reglas.
func NewMockRuleClassifier() *MockRuleClassifier { return &MockRuleClassifier{} }

var taskKeywords = []string{"necesito", "prepara", "haz", "realiza", "encargo", "tarea", "informe", "revisa", "preparar", "hacer"}

var discardKeywords = []string{"ok", "visto", "gracias", "perfecto", "de acuerdo", "entendido"}

// Classify enruta por heurística simple.
func (m *MockRuleClassifier) Classify(_ context.Context, input ClassificationInput) (ClassificationResult, error) {
	lower := strings.ToLower(input.CleanPrompt)

	for _, k := range discardKeywords {
		if strings.Contains(lower, k) {
			return ClassificationResult{
				Decision:   DecisionDiscard,
				Confidence: 0.95,
				Reason:     "cortesía o confirmación breve",
			}, nil
		}
	}

	for _, k := range taskKeywords {
		if strings.Contains(lower, k) {
			return ClassificationResult{
				Decision:   DecisionExtract,
				Confidence: 0.9,
				Reason:     "petición de trabajo detectada",
			}, nil
		}
	}

	return ClassificationResult{
		Decision:   DecisionDiscard,
		Confidence: 0.7,
		Reason:     "sin intención clara",
	}, nil
}
