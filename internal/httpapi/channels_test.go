package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nestorPons/ai-assistant/internal/persistence"
)

type stubWhatsApp struct {
	connected bool
	qr        string
	errMsg    string
}

func (s stubWhatsApp) WhatsAppStatus() (bool, string, string) { return s.connected, s.qr, s.errMsg }
func (s stubWhatsApp) WhatsAppQRPNG() ([]byte, error)          { return nil, nil }

func testServer(cfg ChannelsConfig, wa WhatsAppStatusProvider) *Server {
	s := New(persistence.NewMemoryStore(), slog.Default())
	s.SetChannelsConfig(cfg)
	if wa != nil {
		s.SetWhatsAppProvider(wa)
	}
	return s
}

func getChannels(t *testing.T, s *Server) []ChannelStatus {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/channels/status", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Channels []ChannelStatus `json:"channels"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Channels) != 3 {
		t.Fatalf("channels = %d, want 3", len(body.Channels))
	}
	return body.Channels
}

func byName(ch []ChannelStatus) map[string]ChannelStatus {
	m := map[string]ChannelStatus{}
	for _, c := range ch {
		m[c.Name] = c
	}
	return m
}

func TestChannelsStatusDisabled(t *testing.T) {
	s := testServer(ChannelsConfig{}, nil)
	m := byName(getChannels(t, s))
	for _, name := range []string{"gmail", "telegram", "whatsapp"} {
		if m[name].Enabled {
			t.Errorf("%s enabled = true, want false", name)
		}
		if m[name].Connected {
			t.Errorf("%s connected = true, want false", name)
		}
	}
}

func TestChannelsStatusTelegramConnected(t *testing.T) {
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"result":{"username":"midobot"}}`))
	}))
	defer fake.Close()

	s := testServer(ChannelsConfig{TelegramToken: "abc", TelegramAPIBase: fake.URL}, nil)
	m := byName(getChannels(t, s))
	if !m["telegram"].Enabled || !m["telegram"].Connected {
		t.Errorf("telegram = %+v, want enabled+connected", m["telegram"])
	}
}

func TestChannelsStatusTelegramInvalidToken(t *testing.T) {
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"ok":false,"description":"Unauthorized"}`))
	}))
	defer fake.Close()

	s := testServer(ChannelsConfig{TelegramToken: "bad", TelegramAPIBase: fake.URL}, nil)
	m := byName(getChannels(t, s))
	if !m["telegram"].Enabled || m["telegram"].Connected {
		t.Errorf("telegram = %+v, want enabled+disconnected", m["telegram"])
	}
	if m["telegram"].Error == "" {
		t.Error("telegram error vacío, want mensaje")
	}
}

func TestChannelsStatusGmailConnected(t *testing.T) {
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"emailAddress":"a@example.com"}`))
	}))
	defer fake.Close()

	s := testServer(ChannelsConfig{GmailAccessToken: "tok", GmailPushMode: "poll", GmailAPIBase: fake.URL}, nil)
	m := byName(getChannels(t, s))
	if !m["gmail"].Enabled || !m["gmail"].Connected {
		t.Errorf("gmail = %+v, want enabled+connected", m["gmail"])
	}
}

func TestChannelsStatusWhatsAppConnected(t *testing.T) {
	s := testServer(ChannelsConfig{WhatsAppEnabled: true}, stubWhatsApp{connected: true})
	m := byName(getChannels(t, s))
	if !m["whatsapp"].Enabled || !m["whatsapp"].Connected {
		t.Errorf("whatsapp = %+v, want enabled+connected", m["whatsapp"])
	}
}
