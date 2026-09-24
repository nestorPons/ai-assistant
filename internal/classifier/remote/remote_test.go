package remote

import (
	"context"
	"errors"
	"testing"

	"github.com/nestorPons/ai-assistant/internal/classifier"
	"github.com/nestorPons/ai-assistant/internal/llm"
)

type fakeProvider struct {
	lastReq llm.CompletionRequest
	content string
	err     error
	calls   int
}

func (f *fakeProvider) Complete(_ context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
	f.calls++
	f.lastReq = req
	if f.err != nil {
		return llm.CompletionResponse{}, f.err
	}
	return llm.CompletionResponse{Content: []byte(f.content)}, nil
}

func TestClassifyProceedFast(t *testing.T) {
	p := &fakeProvider{content: `{"decision":"proceed_fast","route":"extract","confidence":0.9,"reason":"tarea"}`}
	c := New(p, "gpt-4o-mini", nil)

	res, err := c.Classify(context.Background(), classifier.ClassificationInput{CleanPrompt: "prepará el informe"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Decision != classifier.DecisionExtract {
		t.Errorf("decision = %v, want extract", res.Decision)
	}
	if res.Confidence != 0.9 {
		t.Errorf("confidence = %v, want 0.9", res.Confidence)
	}
	if len(p.lastReq.JSONSchema) == 0 {
		t.Error("no se envió JSONSchema")
	}
	if p.lastReq.Temperature != 0 {
		t.Errorf("temperature = %v, want 0", p.lastReq.Temperature)
	}
	if p.lastReq.Model != "gpt-4o-mini" {
		t.Errorf("model = %q", p.lastReq.Model)
	}
}

func TestClassifyDeepReviewAndBlock(t *testing.T) {
	cases := []struct {
		name    string
		content string
		review  bool
		blocked bool
	}{
		{"deep_review", `{"decision":"deep_review","route":"discard","confidence":0.4,"reason":"ambiguo"}`, true, false},
		{"block", `{"decision":"block","route":"discard","confidence":0.9,"reason":"spam"}`, false, true},
		{"db_action", `{"decision":"proceed_fast","route":"db_action","confidence":0.8,"reason":"actualizar"}`, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := New(&fakeProvider{content: tc.content}, "m", nil)
			res, err := c.Classify(context.Background(), classifier.ClassificationInput{})
			if err != nil {
				t.Fatal(err)
			}
			if res.NeedsReview != tc.review {
				t.Errorf("needsReview = %v, want %v", res.NeedsReview, tc.review)
			}
			if res.Blocked != tc.blocked {
				t.Errorf("blocked = %v, want %v", res.Blocked, tc.blocked)
			}
		})
	}
}

func TestClassifyInvalidJSON(t *testing.T) {
	c := New(&fakeProvider{content: "no es json"}, "m", nil)
	if _, err := c.Classify(context.Background(), classifier.ClassificationInput{}); err == nil {
		t.Fatal("esperaba error de parseo")
	}
}

func TestClassifyUnknownDecision(t *testing.T) {
	c := New(&fakeProvider{content: `{"decision":"inesperado","route":"x","confidence":1,"reason":""}`}, "m", nil)
	if _, err := c.Classify(context.Background(), classifier.ClassificationInput{}); err == nil {
		t.Fatal("esperaba error de decisión desconocida")
	}
}

func TestClassifyProviderError(t *testing.T) {
	want := llm.NewError(llm.ErrorRateLimit, errors.New("429"))
	c := New(&fakeProvider{err: want}, "m", nil)
	if _, err := c.Classify(context.Background(), classifier.ClassificationInput{}); err == nil {
		t.Fatal("esperaba error del proveedor")
	}
}
