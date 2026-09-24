package pubsub

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nestorPons/ai-assistant/internal/oauth"
)

func newTestClient(baseURL string) *Client {
	c := New(oauth.New(oauth.Config{AccessToken: "token"}))
	c.baseURL = baseURL
	return c
}

func TestPullDecodesNotification(t *testing.T) {
	payload, _ := json.Marshal(Notification{EmailAddress: "me@acme.com", HistoryID: "12345"})
	encoded := base64.StdEncoding.EncodeToString(payload)

	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		fmt.Fprintf(w, `{"receivedMessages":[{"ackId":"a1","message":{"data":%q}}]}`, encoded)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	msgs, err := c.Pull(context.Background(), "projects/p/subscriptions/s", 10)
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/projects/p/subscriptions/s:pull" {
		t.Errorf("path = %q", gotPath)
	}
	if gotAuth != "Bearer token" {
		t.Errorf("auth = %q", gotAuth)
	}
	if len(msgs) != 1 {
		t.Fatalf("mensajes = %d, want 1", len(msgs))
	}
	if msgs[0].AckID != "a1" {
		t.Errorf("ackId = %q", msgs[0].AckID)
	}
	if msgs[0].Notification.HistoryID != "12345" || msgs[0].Notification.EmailAddress != "me@acme.com" {
		t.Errorf("notificación = %+v", msgs[0].Notification)
	}
}

func TestAck(t *testing.T) {
	var gotPath string
	var gotAcks []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		var body struct {
			AckIDs []string `json:"ackIds"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotAcks = body.AckIDs
		fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	if err := c.Ack(context.Background(), "projects/p/subscriptions/s", []string{"a1", "a2"}); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/projects/p/subscriptions/s:ack" {
		t.Errorf("path = %q", gotPath)
	}
	if len(gotAcks) != 2 {
		t.Errorf("ackIds = %v", gotAcks)
	}
}

func TestRunProcessesAndAcks(t *testing.T) {
	var pulls, acked int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/projects/p/subscriptions/s:pull":
			if atomic.AddInt32(&pulls, 1) == 1 {
				payload, _ := json.Marshal(Notification{HistoryID: "1"})
				encoded := base64.StdEncoding.EncodeToString(payload)
				fmt.Fprintf(w, `{"receivedMessages":[{"ackId":"a1","message":{"data":%q}}]}`, encoded)
				return
			}
			fmt.Fprint(w, `{}`)
		case r.URL.Path == "/projects/p/subscriptions/s:ack":
			atomic.StoreInt32(&acked, 1)
			fmt.Fprint(w, `{}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handled := make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() {
		done <- c.Run(ctx, "projects/p/subscriptions/s", func(context.Context) error {
			handled <- struct{}{}
			return nil
		})
	}()

	select {
	case <-handled:
	case <-time.After(2 * time.Second):
		t.Fatal("handler no invocado")
	}

	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadInt32(&acked) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if atomic.LoadInt32(&acked) == 0 {
		t.Fatal("no se hizo ack")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run no terminó tras cancelar")
	}
}

func TestLivePubSubPull(t *testing.T) {
	if os.Getenv("PUBSUB_LIVE_TEST") == "" {
		t.Skip("definir PUBSUB_LIVE_TEST=1 para leer la suscripción real de Pub/Sub")
	}

	env := loadDotEnv(filepath.Join("..", "..", "..", ".env"))
	get := func(key string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return env[key]
	}

	sub := get("GMAIL_PUBSUB_SUBSCRIPTION")
	if sub == "" {
		t.Fatal("falta GMAIL_PUBSUB_SUBSCRIPTION (entorno o .env)")
	}
	clientID, clientSecret, refresh := get("GMAIL_CLIENT_ID"), get("GMAIL_CLIENT_SECRET"), get("GMAIL_REFRESH_TOKEN")
	if clientID == "" || clientSecret == "" || refresh == "" {
		t.Fatal("faltan GMAIL_CLIENT_ID/SECRET/REFRESH_TOKEN")
	}

	c := New(oauth.New(oauth.Config{ClientID: clientID, ClientSecret: clientSecret, RefreshToken: refresh}))
	c.returnImmediately = true

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	msgs, err := c.Pull(ctx, sub, 5)
	if err != nil {
		t.Fatalf("pull falló: %v", err)
	}
	t.Logf("suscripción: %s", sub)
	t.Logf("mensajes recibidos: %d", len(msgs))
	for _, m := range msgs {
		t.Logf("ackId=%s email=%s historyId=%s", m.AckID, m.Notification.EmailAddress, m.Notification.HistoryID)
	}
	if len(msgs) == 0 {
		t.Log("sin mensajes pendientes (normal si nadie publicó cambios aún)")
	}
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
