// Package oauth provee un TokenSource que renueva access tokens de Google a
// partir de un refresh token (OAuth 2.0). Es reutilizable por los distintos
// clientes REST (Gmail, Pub/Sub) que comparten las mismas credenciales.
package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	defaultTokenURL    = "https://oauth2.googleapis.com/token"
	tokenRefreshMargin = 30 * time.Second
)

// Config configura el TokenSource.
type Config struct {
	AccessToken  string
	ClientID     string
	ClientSecret string
	RefreshToken string
	TokenURL     string
	HTTPClient   *http.Client
}

// TokenSource entrega access tokens válidos, refrescándolos cuando expiran.
type TokenSource struct {
	accessToken  string
	clientID     string
	clientSecret string
	refreshToken string
	tokenURL     string
	client       *http.Client

	mu          sync.Mutex
	token       string
	tokenExpiry time.Time
}

// New crea un TokenSource.
func New(cfg Config) *TokenSource {
	if cfg.TokenURL == "" {
		cfg.TokenURL = defaultTokenURL
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &TokenSource{
		accessToken:  cfg.AccessToken,
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		refreshToken: cfg.RefreshToken,
		tokenURL:     cfg.TokenURL,
		client:       cfg.HTTPClient,
	}
}

// Token devuelve un access token válido.
func (t *TokenSource) Token(ctx context.Context) (string, error) {
	if strings.TrimSpace(t.refreshToken) == "" {
		return t.accessToken, nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.token != "" && time.Now().Before(t.tokenExpiry.Add(-tokenRefreshMargin)) {
		return t.token, nil
	}
	return t.refresh(ctx)
}

func (t *TokenSource) refresh(ctx context.Context) (string, error) {
	form := url.Values{}
	form.Set("client_id", t.clientID)
	form.Set("client_secret", t.clientSecret)
	form.Set("refresh_token", t.refreshToken)
	form.Set("grant_type", "refresh_token")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}

	var body struct {
		AccessToken      string `json:"access_token"`
		ExpiresIn        int    `json:"expires_in"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK || body.AccessToken == "" {
		msg := body.ErrorDescription
		if msg == "" {
			msg = body.Error
		}
		if msg == "" {
			msg = strings.TrimSpace(string(data))
		}
		return "", fmt.Errorf("refresh token status %d: %s", resp.StatusCode, msg)
	}

	t.token = body.AccessToken
	if body.ExpiresIn > 0 {
		t.tokenExpiry = time.Now().Add(time.Duration(body.ExpiresIn) * time.Second)
	} else {
		t.tokenExpiry = time.Now().Add(time.Hour)
	}
	return t.token, nil
}
