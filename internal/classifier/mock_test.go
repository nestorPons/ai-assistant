package classifier

import (
	"context"
	"testing"
)

func TestMockRuleClassifier(t *testing.T) {
	m := NewMockRuleClassifier()
	res, err := m.Classify(context.Background(), ClassificationInput{CleanPrompt: "necesito que prepares un informe"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Decision != DecisionExtract {
		t.Errorf("esperaba extract, fue %v", res.Decision)
	}

	res, _ = m.Classify(context.Background(), ClassificationInput{CleanPrompt: "gracias"})
	if res.Decision != DecisionDiscard {
		t.Errorf("esperaba discard, fue %v", res.Decision)
	}
}
