package classifier

import (
	"context"
	"errors"
	"testing"
)

type stubClassifier struct {
	res    ClassificationResult
	err    error
	called int
}

func (s *stubClassifier) Classify(_ context.Context, _ ClassificationInput) (ClassificationResult, error) {
	s.called++
	return s.res, s.err
}

func TestChainUsesPrimaryWhenOK(t *testing.T) {
	primary := &stubClassifier{res: ClassificationResult{Decision: DecisionExtract}}
	fallback := &stubClassifier{res: ClassificationResult{Decision: DecisionDiscard}}

	res, err := NewChain(nil, primary, fallback).Classify(context.Background(), ClassificationInput{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Decision != DecisionExtract {
		t.Errorf("decision = %v, want extract", res.Decision)
	}
	if fallback.called != 0 {
		t.Error("el fallback no debería ejecutarse")
	}
}

func TestChainFallsBackOnError(t *testing.T) {
	primary := &stubClassifier{err: errors.New("rate limit")}
	fallback := &stubClassifier{res: ClassificationResult{Decision: DecisionDBAction}}

	res, err := NewChain(nil, primary, fallback).Classify(context.Background(), ClassificationInput{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Decision != DecisionDBAction {
		t.Errorf("decision = %v, want db_action", res.Decision)
	}
	if fallback.called != 1 {
		t.Errorf("fallback llamado = %d, want 1", fallback.called)
	}
}

func TestChainAllFailGoesToReview(t *testing.T) {
	primary := &stubClassifier{err: errors.New("boom")}
	secondary := &stubClassifier{err: errors.New("boom")}

	res, err := NewChain(nil, primary, secondary).Classify(context.Background(), ClassificationInput{})
	if err != nil {
		t.Fatalf("no debe devolver error: %v", err)
	}
	if !res.NeedsReview {
		t.Error("debería marcarse para revisión")
	}
}

func TestChainRespectsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	primary := &stubClassifier{err: errors.New("boom")}
	fallback := &stubClassifier{res: ClassificationResult{Decision: DecisionExtract}}

	if _, err := NewChain(nil, primary, fallback).Classify(ctx, ClassificationInput{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if fallback.called != 0 {
		t.Error("no debe ejecutar fallback con contexto cancelado")
	}
}
