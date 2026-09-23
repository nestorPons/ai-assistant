package jev

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nestorPons/ai-assistant/internal/classifier"
)

func TestClassifyMapping(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		want    classifier.Decision
		review  bool
		blocked bool
		wantErr bool
	}{
		{
			name: "extract interno",
			body: `{"code":0,"message":"ok","data":{"decision":"extract","confidence":0.9,"guidance":"hay tarea"}}`,
			want: classifier.DecisionExtract,
		},
		{
			name: "proceed_fast con route",
			body: `{"code":0,"message":"ok","data":{"decision":"proceed_fast","confidence":0.8,"route":"extract"}}`,
			want: classifier.DecisionExtract,
		},
		{
			name:   "deep_review",
			body:   `{"code":0,"message":"ok","data":{"decision":"deep_review","confidence":0.4}}`,
			want:   classifier.DecisionDiscard,
			review: true,
		},
		{
			name:    "block",
			body:    `{"code":0,"message":"ok","data":{"decision":"block","confidence":0.9}}`,
			want:    classifier.DecisionDiscard,
			blocked: true,
		},
		{
			name:    "code distinto de cero",
			body:    `{"code":1,"message":"error"}`,
			wantErr: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/decisions/route" {
					t.Errorf("path inesperado: %s", r.URL.Path)
				}
				io.WriteString(w, c.body)
			}))
			defer srv.Close()

			p := New(srv.URL, "token", 0)
			res, err := p.Classify(context.Background(), classifier.ClassificationInput{CleanPrompt: "hola"})
			if c.wantErr {
				if err == nil {
					t.Fatal("esperaba error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if res.Decision != c.want {
				t.Errorf("decision = %v, want %v", res.Decision, c.want)
			}
			if res.NeedsReview != c.review {
				t.Errorf("needsReview = %v, want %v", res.NeedsReview, c.review)
			}
			if res.Blocked != c.blocked {
				t.Errorf("blocked = %v, want %v", res.Blocked, c.blocked)
			}
		})
	}
}

func TestMapDecision(t *testing.T) {
	r, err := mapDecision(routeData{Decision: "block", Confidence: 0.9})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Blocked {
		t.Error("block no marcado como bloqueado")
	}

	if _, err := mapDecision(routeData{Decision: "inesperado"}); err == nil {
		t.Error("decisión desconocida debería fallar")
	}
}
