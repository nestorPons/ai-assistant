package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/nestorPons/ai-assistant/internal/oauth"
)

// ChannelStatus describe el estado público de un canal de ingesta para el panel.
type ChannelStatus struct {
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	Connected bool   `json:"connected"`
	Mode      string `json:"mode,omitempty"`
	Error     string `json:"error,omitempty"`
	Detail    string `json:"detail,omitempty"`
}

// ChannelsConfig agrupa lo necesario para comprobar cada canal sin exponer secretos.
type ChannelsConfig struct {
	TelegramToken   string
	TelegramAPIBase string // override en tests; "" = api.telegram.org

	GmailAccessToken  string
	GmailClientID     string
	GmailClientSecret string
	GmailRefreshToken string
	GmailPushMode     string
	GmailTokenURL     string // override en tests (oauth refresh)
	GmailAPIBase      string // override en tests; "" = gmail.googleapis.com

	WhatsAppEnabled bool

	HTTPClient *http.Client
}

// SetChannelsConfig adjunta la configuración para el endpoint agregado.
func (s *Server) SetChannelsConfig(cfg ChannelsConfig) {
	s.channels = cfg
}

func (s *Server) channelsStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	channels := []ChannelStatus{
		s.telegramStatus(ctx),
		s.gmailStatus(ctx),
		s.whatsappChannelStatus(),
	}
	s.write(w, http.StatusOK, map[string]any{"channels": channels})
}

func (s *Server) httpClient() *http.Client {
	if s.channels.HTTPClient != nil {
		return s.channels.HTTPClient
	}
	return &http.Client{Timeout: 5 * time.Second}
}

func (s *Server) telegramStatus(ctx context.Context) ChannelStatus {
	token := strings.TrimSpace(s.channels.TelegramToken)
	if token == "" {
		return ChannelStatus{Name: "telegram", Enabled: false, Error: "no configurado (TELEGRAM_BOT_TOKEN vacío)"}
	}

	base := strings.TrimRight(s.channels.TelegramAPIBase, "/")
	if base == "" {
		base = "https://api.telegram.org"
	}

	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, base+"/bot"+token+"/getMe", nil)
	if err != nil {
		return ChannelStatus{Name: "telegram", Enabled: true, Error: err.Error()}
	}

	resp, err := s.httpClient().Do(req)
	if err != nil {
		return ChannelStatus{Name: "telegram", Enabled: true, Mode: "poll", Error: "sin conexión con Telegram"}
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return ChannelStatus{Name: "telegram", Enabled: true, Mode: "poll", Error: "respuesta ilegible de Telegram"}
	}

	var body struct {
		OK          bool `json:"ok"`
		Description string `json:"description"`
		Result      struct {
			Username string `json:"username"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return ChannelStatus{Name: "telegram", Enabled: true, Mode: "poll", Error: "respuesta inválida de Telegram"}
	}
	if !body.OK {
		msg := body.Description
		if msg == "" {
			msg = fmt.Sprintf("status %d", resp.StatusCode)
		}
		return ChannelStatus{Name: "telegram", Enabled: true, Mode: "poll", Error: msg}
	}

	st := ChannelStatus{Name: "telegram", Enabled: true, Connected: true, Mode: "poll"}
	if body.Result.Username != "" {
		st.Detail = "@" + body.Result.Username
	}
	return st
}

func (s *Server) gmailStatus(ctx context.Context) ChannelStatus {
	c := s.channels
	hasRefresh := strings.TrimSpace(c.GmailClientID) != "" &&
		strings.TrimSpace(c.GmailClientSecret) != "" &&
		strings.TrimSpace(c.GmailRefreshToken) != ""
	hasAccess := strings.TrimSpace(c.GmailAccessToken) != ""
	if !hasRefresh && !hasAccess {
		return ChannelStatus{Name: "gmail", Enabled: false, Error: "no configurado (faltan credenciales OAuth)"}
	}

	mode := strings.TrimSpace(c.GmailPushMode)
	if mode == "" {
		mode = "poll"
	}

	tokens := oauth.New(oauth.Config{
		AccessToken:  c.GmailAccessToken,
		ClientID:     c.GmailClientID,
		ClientSecret: c.GmailClientSecret,
		RefreshToken: c.GmailRefreshToken,
		TokenURL:     c.GmailTokenURL,
		HTTPClient:   s.httpClient(),
	})

	tokCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	token, err := tokens.Token(tokCtx)
	if err != nil || strings.TrimSpace(token) == "" {
		msg := "no se pudo obtener access token"
		if err != nil {
			msg = err.Error()
		}
		return ChannelStatus{Name: "gmail", Enabled: true, Mode: mode, Error: msg}
	}

	base := strings.TrimRight(c.GmailAPIBase, "/")
	if base == "" {
		base = "https://gmail.googleapis.com/gmail/v1"
	}

	req, err := http.NewRequestWithContext(tokCtx, http.MethodGet, base+"/users/me/profile", nil)
	if err != nil {
		return ChannelStatus{Name: "gmail", Enabled: true, Mode: mode, Error: err.Error()}
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := s.httpClient().Do(req)
	if err != nil {
		return ChannelStatus{Name: "gmail", Enabled: true, Mode: mode, Error: "sin conexión con Gmail"}
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode != http.StatusOK {
		return ChannelStatus{Name: "gmail", Enabled: true, Mode: mode, Error: fmt.Sprintf("gmail profile status %d", resp.StatusCode)}
	}
	return ChannelStatus{Name: "gmail", Enabled: true, Connected: true, Mode: mode}
}

func (s *Server) whatsappChannelStatus() ChannelStatus {
	if !s.channels.WhatsAppEnabled && s.whatsapp == nil {
		return ChannelStatus{Name: "whatsapp", Enabled: false, Error: "no habilitado (WHATSAPP_ENABLED=false)"}
	}
	if s.whatsapp == nil {
		return ChannelStatus{Name: "whatsapp", Enabled: true, Error: "sin proveedor de estado"}
	}
	connected, _, errMsg := s.whatsapp.WhatsAppStatus()
	st := ChannelStatus{Name: "whatsapp", Enabled: true, Connected: connected, Mode: "qr"}
	if errMsg != "" && !connected {
		st.Error = errMsg
	}
	return st
}
