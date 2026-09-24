package gmail

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nestorPons/ai-assistant/internal/domain"
	"github.com/nestorPons/ai-assistant/internal/oauth"
	"github.com/nestorPons/ai-assistant/internal/persistence"
)

func newTestSource(t *testing.T, baseURL string, store CursorStore) *Source {
	t.Helper()
	s := New(Config{AccessToken: "token", Sync: store})
	s.baseURL = baseURL
	return s
}

func TestListIDsLastMessageEmail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/messages"):
			fmt.Fprint(w, `{"messages":[{"id":"latest"},{"id":"older"}]}`)
		case strings.HasSuffix(r.URL.Path, "/messages/latest"):
			fmt.Fprint(w, `{"id":"latest","snippet":"nos vemos el martes","payload":{"headers":[{"name":"From","value":"Ana Lopez <ana.lopez@acme.com>"},{"name":"Subject","value":"Reunion"}]}}`)
		case strings.HasSuffix(r.URL.Path, "/messages/older"):
			fmt.Fprint(w, `{"id":"older","snippet":"ok","payload":{"headers":[{"name":"From","value":"viejo@acme.com"},{"name":"Subject","value":"Antiguo"}]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	s := newTestSource(t, srv.URL, nil)

	ids, err := s.listIDs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != "latest" {
		t.Fatalf("ids = %v, se esperaba el más reciente primero", ids)
	}

	msg, err := s.getMessage(context.Background(), ids[0])
	if err != nil {
		t.Fatal(err)
	}
	if msg.ClientIdentifier != "ana.lopez@acme.com" {
		t.Errorf("email = %q, want %q", msg.ClientIdentifier, "ana.lopez@acme.com")
	}
	if msg.ClientName != "Ana Lopez" {
		t.Errorf("name = %q, want %q", msg.ClientName, "Ana Lopez")
	}
	if msg.ID != "latest" {
		t.Errorf("id = %q, want latest", msg.ID)
	}
}

func TestExtractEmail(t *testing.T) {
	cases := map[string]string{
		"Ana Lopez <ana.lopez@acme.com>": "ana.lopez@acme.com",
		"plain@acme.com":                 "plain@acme.com",
		"sin email":                      "",
	}
	for in, want := range cases {
		if got := extractEmail(in); got != want {
			t.Errorf("extractEmail(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSyncBaselineFirstRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/users/me/profile" {
			fmt.Fprint(w, `{"emailAddress":"me@acme.com","historyId":"100"}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	store := persistence.NewMemoryStore()
	s := newTestSource(t, srv.URL, store)

	out := make(chan domain.IncomingMessage, 10)
	if err := s.Sync(context.Background(), out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Errorf("baseline no debe emitir mensajes, got %d", len(out))
	}

	state, err := store.GetSyncState(context.Background(), domain.SourceGmail)
	if err != nil {
		t.Fatal(err)
	}
	if state.Cursor != "100" {
		t.Errorf("cursor = %q, want 100", state.Cursor)
	}
}

func TestSyncIncremental(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/users/me/history":
			if r.URL.Query().Get("startHistoryId") != "100" {
				t.Errorf("startHistoryId = %q, want 100", r.URL.Query().Get("startHistoryId"))
			}
			fmt.Fprint(w, `{"history":[{"messagesAdded":[{"message":{"id":"m1"}}]}],"historyId":"200"}`)
		case r.URL.Path == "/users/me/messages/m1":
			fmt.Fprint(w, `{"id":"m1","snippet":"hola","payload":{"headers":[{"name":"From","value":"Ana <ana@acme.com>"}]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	store := persistence.NewMemoryStore()
	store.SaveSyncState(context.Background(), &domain.SyncState{Source: domain.SourceGmail, Cursor: "100"})

	s := newTestSource(t, srv.URL, store)
	out := make(chan domain.IncomingMessage, 10)
	if err := s.Sync(context.Background(), out); err != nil {
		t.Fatal(err)
	}

	if len(out) != 1 {
		t.Fatalf("mensajes emitidos = %d, want 1", len(out))
	}
	if got := (<-out).ID; got != "m1" {
		t.Errorf("id = %q, want m1", got)
	}
	state, _ := store.GetSyncState(context.Background(), domain.SourceGmail)
	if state.Cursor != "200" {
		t.Errorf("cursor = %q, want 200", state.Cursor)
	}
}

func TestSyncHistoryExpiredResync(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/users/me/history":
			w.WriteHeader(http.StatusNotFound)
		case r.URL.Path == "/users/me/messages":
			fmt.Fprint(w, `{"messages":[{"id":"m2"}]}`)
		case r.URL.Path == "/users/me/messages/m2":
			fmt.Fprint(w, `{"id":"m2","snippet":"nuevo","payload":{"headers":[{"name":"From","value":"ana@acme.com"}]}}`)
		case r.URL.Path == "/users/me/profile":
			fmt.Fprint(w, `{"historyId":"300"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	store := persistence.NewMemoryStore()
	store.SaveSyncState(context.Background(), &domain.SyncState{Source: domain.SourceGmail, Cursor: "100"})

	s := newTestSource(t, srv.URL, store)
	out := make(chan domain.IncomingMessage, 10)
	if err := s.Sync(context.Background(), out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("mensajes emitidos = %d, want 1", len(out))
	}
	state, _ := store.GetSyncState(context.Background(), domain.SourceGmail)
	if state.Cursor != "300" {
		t.Errorf("cursor = %q, want 300", state.Cursor)
	}
}

func TestEnsureWatchBaseline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/me/watch" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `{"historyId":"50","expiration":"1893456000000"}`)
	}))
	defer srv.Close()

	store := persistence.NewMemoryStore()
	s := New(Config{AccessToken: "token", TopicName: "projects/p/topics/gmail-push", Sync: store})
	s.baseURL = srv.URL

	if err := s.ensureWatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	state, err := store.GetSyncState(context.Background(), domain.SourceGmail)
	if err != nil {
		t.Fatal(err)
	}
	if state.Cursor != "50" {
		t.Errorf("cursor = %q, want 50", state.Cursor)
	}
	if state.ExpiresAt.IsZero() {
		t.Error("expiración no guardada")
	}
}

func TestLiveGmailLastMessage(t *testing.T) {
	if os.Getenv("GMAIL_LIVE_TEST") == "" {
		t.Skip("definir GMAIL_LIVE_TEST=1 para consultar la API real de Gmail")
	}

	env := loadDotEnv(filepath.Join("..", "..", "..", ".env"))
	get := func(key string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return env[key]
	}

	query := get("GMAIL_TEST_QUERY")
	if query == "" {
		query = "in:inbox"
	}

	clientID, clientSecret, refresh := get("GMAIL_CLIENT_ID"), get("GMAIL_CLIENT_SECRET"), get("GMAIL_REFRESH_TOKEN")
	access := get("GMAIL_ACCESS_TOKEN")

	cfg := Config{Query: query}
	if clientID != "" && clientSecret != "" && refresh != "" {
		cfg.Tokens = oauth.New(oauth.Config{ClientID: clientID, ClientSecret: clientSecret, RefreshToken: refresh})
	} else if access != "" {
		cfg.AccessToken = access
	} else {
		t.Fatal("configurá GMAIL_CLIENT_ID/SECRET/REFRESH_TOKEN o GMAIL_ACCESS_TOKEN (entorno o .env)")
	}

	s := New(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ids, err := s.listIDs(ctx)
	if err != nil {
		t.Fatalf("listIDs falló: %v", err)
	}
	if len(ids) == 0 {
		t.Fatalf("no hay mensajes para la query %q", query)
	}

	msg, err := s.getMessage(ctx, ids[0])
	if err != nil {
		t.Fatalf("getMessage falló: %v", err)
	}
	if msg.ClientIdentifier == "" {
		t.Fatal("el último mensaje no tiene email de remitente")
	}

	t.Logf("último mensaje: id=%s email=%s nombre=%q asunto/contenido=%q",
		msg.ID, msg.ClientIdentifier, msg.ClientName, msg.RawContent)
}

func loadDotEnv(path string) map[string]string {
	out := map[string]string{}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		out[key] = val
	}
	return out
}
