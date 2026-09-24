// Package gmail implementa un listener de Gmail mediante la API REST. Soporta
// polling por query y sincronización incremental por History API (Pub/Sub).
// El SDK externo se aísla aquí; el resto del sistema solo ve IncomingMessage.
package gmail

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/nestorPons/ai-assistant/internal/domain"
	"github.com/nestorPons/ai-assistant/internal/oauth"
)

const (
	defaultBaseURL    = "https://gmail.googleapis.com/gmail/v1"
	defaultFallback   = 5 * time.Minute
	defaultWatchRenew = 6 * time.Hour
)

// CursorStore persiste el cursor de sincronización incremental.
type CursorStore interface {
	GetSyncState(ctx context.Context, source domain.Source) (*domain.SyncState, error)
	SaveSyncState(ctx context.Context, state *domain.SyncState) error
}

// Source hace polling/sincronización de la bandeja de Gmail.
type Source struct {
	tokens    *oauth.TokenSource
	query     string
	poll      time.Duration
	topicName string
	fallback  time.Duration
	backlog   bool
	cursor    CursorStore
	baseURL   string
	client    *http.Client
	logger    *slog.Logger

	syncMu sync.Mutex
}

// Config ajusta el listener de Gmail.
type Config struct {
	Tokens      *oauth.TokenSource
	AccessToken string
	Query       string
	Poll        time.Duration
	TopicName   string
	Fallback    time.Duration
	// Backlog procesa los no leídos actuales en el primer arranque (sin cursor).
	Backlog bool
	Sync    CursorStore
	Logger  *slog.Logger
}

// New crea una fuente Gmail.
func New(cfg Config) *Source {
	if cfg.Poll <= 0 {
		cfg.Poll = 60 * time.Second
	}
	if cfg.Query == "" {
		cfg.Query = "is:unread"
	}
	if cfg.Fallback <= 0 {
		cfg.Fallback = defaultFallback
	}
	if cfg.Tokens == nil {
		cfg.Tokens = oauth.New(oauth.Config{AccessToken: cfg.AccessToken})
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Source{
		tokens:    cfg.Tokens,
		query:     cfg.Query,
		poll:      cfg.Poll,
		topicName: cfg.TopicName,
		fallback:  cfg.Fallback,
		backlog:   cfg.Backlog,
		cursor:    cfg.Sync,
		baseURL:   defaultBaseURL,
		client:    &http.Client{Timeout: 30 * time.Second},
		logger:    cfg.Logger,
	}
}

func (s *Source) authToken(ctx context.Context) (string, error) {
	return s.tokens.Token(ctx)
}

// Name identifica el canal.
func (s *Source) Name() string { return "gmail" }

// EnsureWatch registra el watch de Gmail hacia el topic de Pub/Sub configurado
// (y renueva el baseline si no hay cursor). Requiere TopicName.
func (s *Source) EnsureWatch(ctx context.Context) error {
	return s.ensureWatch(ctx)
}

// Start ejecuta el bucle correspondiente: incremental (watch + Pub/Sub) si hay
// topic configurado, o polling por query en caso contrario.
func (s *Source) Start(ctx context.Context, out chan<- domain.IncomingMessage) error {
	if s.topicName != "" && s.cursor != nil {
		return s.runIncremental(ctx, out)
	}
	return s.runPoll(ctx, out)
}

func (s *Source) runPoll(ctx context.Context, out chan<- domain.IncomingMessage) error {
	ticker := time.NewTicker(s.poll)
	defer ticker.Stop()

	for {
		if err := s.pollOnce(ctx, out); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			s.logger.Error("gmail poll", "error", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (s *Source) runIncremental(ctx context.Context, out chan<- domain.IncomingMessage) error {
	if err := s.ensureWatch(ctx); err != nil && ctx.Err() == nil {
		s.logger.Error("gmail watch inicial", "error", err)
	}
	go s.renewWatchLoop(ctx)

	ticker := time.NewTicker(s.fallback)
	defer ticker.Stop()

	if err := s.Sync(ctx, out); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		s.logger.Error("gmail sync inicial", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := s.Sync(ctx, out); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				s.logger.Error("gmail sync de respaldo", "error", err)
			}
		}
	}
}

func (s *Source) renewWatchLoop(ctx context.Context) {
	ticker := time.NewTicker(defaultWatchRenew)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.ensureWatch(ctx); err != nil && ctx.Err() == nil {
				s.logger.Error("gmail watch renovación", "error", err)
			}
		}
	}
}

func (s *Source) pollOnce(ctx context.Context, out chan<- domain.IncomingMessage) error {
	ids, err := s.listIDs(ctx)
	if err != nil {
		return err
	}
	return s.emit(ctx, out, ids)
}

func (s *Source) emit(ctx context.Context, out chan<- domain.IncomingMessage, ids []string) error {
	for _, id := range ids {
		msg, err := s.getMessage(ctx, id)
		if err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case out <- msg:
		}
	}
	return nil
}

func (s *Source) listIDs(ctx context.Context) ([]string, error) {
	token, err := s.authToken(ctx)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/users/me/messages?q=%s&maxResults=25", s.baseURL, s.query)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gmail list status %d", resp.StatusCode)
	}

	var body struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(body.Messages))
	for _, m := range body.Messages {
		ids = append(ids, m.ID)
	}
	return ids, nil
}

func (s *Source) getMessage(ctx context.Context, id string) (domain.IncomingMessage, error) {
	token, err := s.authToken(ctx)
	if err != nil {
		return domain.IncomingMessage{}, err
	}

	url := fmt.Sprintf("%s/users/me/messages/%s?format=full", s.baseURL, id)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := s.client.Do(req)
	if err != nil {
		return domain.IncomingMessage{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return domain.IncomingMessage{}, fmt.Errorf("gmail get status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return domain.IncomingMessage{}, err
	}

	var msg struct {
		ID      string    `json:"id"`
		Snippet string    `json:"snippet"`
		Payload gmailPart `json:"payload"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return domain.IncomingMessage{}, err
	}

	var from, subject string
	for _, h := range msg.Payload.Headers {
		switch strings.ToLower(h.Name) {
		case "from":
			from = h.Value
		case "subject":
			subject = h.Value
		}
	}

	email := extractEmail(from)
	if email == "" {
		return domain.IncomingMessage{}, errors.New("gmail: remitente sin email")
	}

	body := extractPlainText(msg.Payload)
	if body == "" {
		body = msg.Snippet
	}
	content := strings.TrimSpace(subject + "\n" + body)

	return domain.IncomingMessage{
		ID:               msg.ID,
		Source:           domain.SourceGmail,
		ClientIdentifier: email,
		ClientName:       extractName(from),
		RawContent:       content,
		ReceivedAt:       time.Now().UTC(),
	}, nil
}

// gmailPart modela la estructura payload/parts de la API Gmail (format=full).
type gmailPart struct {
	MimeType string `json:"mimeType"`
	Filename string `json:"filename"`
	Body     struct {
		Size int    `json:"size"`
		Data string `json:"data"`
	} `json:"body"`
	Parts   []gmailPart `json:"parts"`
	Headers []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"headers"`
}

// extractPlainText devuelve el texto plano del mensaje, recorriendo las partes
// MIME y prefiriendo text/plain sobre text/html. Devuelve "" si no hay cuerpo.
func extractPlainText(p gmailPart) string {
	if p.MimeType == "text/plain" {
		if s := decodeBody(p.Body.Data); s != "" {
			return s
		}
	}
	var html string
	for _, part := range p.Parts {
		if s := extractPlainText(part); s != "" {
			return s
		}
		if part.MimeType == "text/html" && html == "" {
			html = decodeBody(part.Body.Data)
		}
	}
	return html
}

// decodeBody decodifica el campo body.data (base64url) de la API Gmail.
func decodeBody(data string) string {
	if data == "" {
		return ""
	}
	dec, err := base64.URLEncoding.DecodeString(data)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(dec))
}

var emailRe = regexp.MustCompile(`<([^>]+)>`)

func extractEmail(from string) string {
	if m := emailRe.FindStringSubmatch(from); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	// fallback: la dirección sin nombre
	if strings.Contains(from, "@") {
		return strings.TrimSpace(strings.Split(from, " ")[0])
	}
	return ""
}

func extractName(from string) string {
	if i := strings.Index(from, "<"); i > 0 {
		return strings.TrimSpace(from[:i])
	}
	return from
}
