package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nestorPons/ai-assistant/internal/domain"
	"github.com/nestorPons/ai-assistant/internal/persistence"
)

func TestToIncoming(t *testing.T) {
	ts := time.Unix(1700000000, 0).UTC()
	base := update{
		UpdateID: 10,
		Message: &message{
			MessageID: 1,
			From:      &user{ID: 42, FirstName: "Ana", LastName: "López"},
			Date:      1700000000,
			Text:      "  Prepara el informe urgente  ",
		},
	}

	s := New(Config{Token: "t"})
	msg, ok := s.toIncoming(base)
	if !ok {
		t.Fatal("se esperaba emisión")
	}
	if msg.ID != "10" || msg.Source != domain.SourceTelegram {
		t.Errorf("id/source = %q/%q", msg.ID, msg.Source)
	}
	if msg.ClientIdentifier != "42" || msg.ClientName != "Ana López" {
		t.Errorf("cliente = %q/%q", msg.ClientIdentifier, msg.ClientName)
	}
	if msg.RawContent != "Prepara el informe urgente" {
		t.Errorf("contenido = %q", msg.RawContent)
	}
	if !msg.ReceivedAt.Equal(ts) {
		t.Errorf("received_at = %v, want %v", msg.ReceivedAt, ts)
	}

	caption := base
	caption.Message.Text = ""
	caption.Message.Caption = "Descripción de la foto"
	if msg, ok := s.toIncoming(caption); !ok || msg.RawContent != "Descripción de la foto" {
		t.Errorf("caption no procesado: ok=%v content=%q", ok, msg.RawContent)
	}

	bot := base
	bot.Message.From.IsBot = true
	if _, ok := s.toIncoming(bot); ok {
		t.Error("los bots no deben emitirse")
	}

	if _, ok := s.toIncoming(update{UpdateID: 1}); ok {
		t.Error("update sin mensaje no debe emitirse")
	}
}

func TestStartEmitsAndPersistsOffset(t *testing.T) {
	var mu sync.Mutex
	var offsets []int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/getUpdates") {
			http.NotFound(w, r)
			return
		}
		var req struct {
			Offset int64 `json:"offset"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		mu.Lock()
		offsets = append(offsets, req.Offset)
		first := len(offsets) == 1
		mu.Unlock()

		if first {
			fmt.Fprint(w, `{"ok":true,"result":[{"update_id":10,"message":{"message_id":1,"from":{"id":42,"is_bot":false,"first_name":"Ana"},"date":1700000000,"text":"Prepara el informe para hoy"}}]}`)
			return
		}
		fmt.Fprint(w, `{"ok":true,"result":[]}`)
	}))
	defer srv.Close()

	store := persistence.NewMemoryStore()
	s := New(Config{
		Token:   "TEST",
		APIBase: srv.URL,
		Poll:    time.Second,
		Cursor:  store,
	})

	out := make(chan domain.IncomingMessage, 4)
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- s.Start(ctx, out) }()

	select {
	case msg := <-out:
		if msg.ID != "10" || msg.ClientIdentifier != "42" {
			t.Fatalf("mensaje inesperado: %+v", msg)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no se emitió ningún mensaje")
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		state, err := store.GetSyncState(context.Background(), domain.SourceTelegram)
		if err == nil && state.Cursor == "11" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("offset no persistido: state=%+v err=%v", state, err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	seen := append([]int64(nil), offsets...)
	mu.Unlock()
	if len(seen) >= 2 && seen[1] != 11 {
		t.Errorf("segundo offset = %d, want 11", seen[1])
	}

	cancel()
	select {
	case err := <-errCh:
		if err != context.Canceled {
			t.Errorf("error de cierre = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start no terminó al cancelar el contexto")
	}
}

func TestStartResumesFromCursor(t *testing.T) {
	var mu sync.Mutex
	var firstOffset int64 = -1

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Offset int64 `json:"offset"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		mu.Lock()
		if firstOffset < 0 {
			firstOffset = req.Offset
		}
		mu.Unlock()
		fmt.Fprint(w, `{"ok":true,"result":[]}`)
	}))
	defer srv.Close()

	store := persistence.NewMemoryStore()
	_ = store.SaveSyncState(context.Background(), &domain.SyncState{
		Source: domain.SourceTelegram,
		Cursor: "500",
	})

	s := New(Config{Token: "TEST", APIBase: srv.URL, Poll: time.Second, Cursor: store})
	out := make(chan domain.IncomingMessage, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = s.Start(ctx, out) }()

	deadline := time.Now().Add(2 * time.Second)
	for {
		mu.Lock()
		off := firstOffset
		mu.Unlock()
		if off >= 0 {
			if off != 500 {
				t.Errorf("offset inicial = %d, want 500", off)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no se llamó a getUpdates")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
