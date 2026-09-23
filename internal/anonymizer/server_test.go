package anonymizer

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nestorPons/ai-assistant/internal/anonymizer/detect"
)

func newTestServer(token string, ttl time.Duration) *Server {
	d := detect.New(nil, nil, nil)
	e := NewEngine(d, detect.NoopNER{})
	store := NewMemoryMappingStore()
	return NewServer(e, store, Config{Token: token, ProcessingTTL: ttl, MaxBytes: 32 << 10}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func doReq(t *testing.T, s *Server, method, path, token string, body any, out any) *httptest.ResponseRecorder {
	t.Helper()
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if out != nil && rec.Code == http.StatusOK {
		_ = json.Unmarshal(rec.Body.Bytes(), out)
	}
	return rec
}

func TestSanitizeAndRevertRoundTrip(t *testing.T) {
	s := newTestServer("", 5*time.Minute)

	var san SanitizeResponse
	rec := doReq(t, s, http.MethodPost, "/api/sanitize", "", SanitizeRequest{Prompt: "Contacta con maria@example.com"}, &san)
	if rec.Code != http.StatusOK {
		t.Fatalf("sanitize status %d: %s", rec.Code, rec.Body.String())
	}
	if san.RequestID == "" {
		t.Fatal("request_id vacío")
	}
	if strings.Contains(san.CleanPrompt, "maria@example.com") {
		t.Fatalf("PII no ofuscada: %q", san.CleanPrompt)
	}

	var rev RevertResponse
	rec = doReq(t, s, http.MethodPost, "/api/revert", "", RevertRequest{RequestID: san.RequestID, ProcessedText: san.CleanPrompt}, &rev)
	if rec.Code != http.StatusOK {
		t.Fatalf("revert status %d: %s", rec.Code, rec.Body.String())
	}
	if rev.RevertedText != "Contacta con maria@example.com" {
		t.Fatalf("revert incorrecto: %q", rev.RevertedText)
	}

	// La tabla se elimina tras la reversión.
	rec = doReq(t, s, http.MethodPost, "/api/revert", "", RevertRequest{RequestID: san.RequestID, ProcessedText: san.CleanPrompt}, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("segunda revert debería ser 404, fue %d", rec.Code)
	}
}

func TestSanitizeRequiresAuth(t *testing.T) {
	s := newTestServer("secreto", 5*time.Minute)
	rec := doReq(t, s, http.MethodPost, "/api/sanitize", "", SanitizeRequest{Prompt: "hola"}, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("sin token debería devolver 401, fue %d", rec.Code)
	}
}

func TestSanitizeRejectsUnknownToken(t *testing.T) {
	s := newTestServer("", 5*time.Minute)
	var san SanitizeResponse
	doReq(t, s, http.MethodPost, "/api/sanitize", "", SanitizeRequest{Prompt: "hola a maria@example.com"}, &san)

	rec := doReq(t, s, http.MethodPost, "/api/revert", "", RevertRequest{RequestID: san.RequestID, ProcessedText: "{{DESCONOCIDO_1}}"}, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("token desconocido debería dar 400, fue %d", rec.Code)
	}
}

func TestSanitizeMissingPrompt(t *testing.T) {
	s := newTestServer("", 5*time.Minute)
	rec := doReq(t, s, http.MethodPost, "/api/sanitize", "", SanitizeRequest{}, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("prompt vacío debería dar 400, fue %d", rec.Code)
	}
}

func TestMappingTTLExpiry(t *testing.T) {
	store := NewMemoryMappingStore()
	ctx := context.Background()
	if err := store.Save(ctx, "r1", MappingTable{"{{X_1}}": "valor"}, 10*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	if _, err := store.Load(ctx, "r1"); err != ErrMappingNotFound {
		t.Fatalf("debería expirar, error=%v", err)
	}
}
