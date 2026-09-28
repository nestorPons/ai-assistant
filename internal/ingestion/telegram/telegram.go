// Package telegram implementa un canal de entrada mediante la Bot API de
// Telegram en modo long polling. El HTTP externo se aísla aquí; el resto del
// sistema solo ve domain.IncomingMessage. El filtro de la lista blanca lo aplica
// el pipeline usando la tabla clients gestionada desde el dashboard.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nestorPons/ai-assistant/internal/domain"
	"github.com/nestorPons/ai-assistant/internal/persistence"
)

const (
	defaultAPIBase  = "https://api.telegram.org"
	defaultPoll     = 25 * time.Second
	defaultRetry    = 3 * time.Second
	maxResponseSize = 1 << 20
)

// CursorStore persiste el offset de getUpdates para no reprocesar updates.
type CursorStore interface {
	GetSyncState(ctx context.Context, source domain.Source) (*domain.SyncState, error)
	SaveSyncState(ctx context.Context, state *domain.SyncState) error
}

// Config ajusta el conector de Telegram.
type Config struct {
	// Token del bot (BotFather). Obligatorio.
	Token string
	// APIBase permite apuntar a un servidor de pruebas; por defecto la API oficial.
	APIBase string
	// Poll es el timeout de long polling de getUpdates.
	Poll   time.Duration
	Cursor CursorStore
	Logger *slog.Logger
}

// Source hace long polling de la Bot API y emite mensajes de usuario nuevos.
type Source struct {
	token   string
	apiBase string
	poll    int
	cursor  CursorStore
	client  *http.Client
	logger  *slog.Logger
}

// New crea una fuente de Telegram.
func New(cfg Config) *Source {
	if cfg.APIBase == "" {
		cfg.APIBase = defaultAPIBase
	}
	if cfg.Poll <= 0 {
		cfg.Poll = defaultPoll
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Source{
		token:   cfg.Token,
		apiBase: strings.TrimRight(cfg.APIBase, "/"),
		poll:    int(cfg.Poll / time.Second),
		cursor:  cfg.Cursor,
		client:  &http.Client{Timeout: cfg.Poll + 15*time.Second},
		logger:  cfg.Logger,
	}
}

// Name identifica el canal.
func (s *Source) Name() string { return "telegram" }

// Start ejecuta el bucle de long polling hasta que el contexto se cancela.
func (s *Source) Start(ctx context.Context, out chan<- domain.IncomingMessage) error {
	if s.token == "" {
		return errors.New("telegram: token vacío")
	}

	offset := s.loadOffset(ctx)
	for {
		updates, err := s.getUpdates(ctx, offset)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			s.logger.Error("telegram getUpdates", "error", err)
			if !sleep(ctx, defaultRetry) {
				return ctx.Err()
			}
			continue
		}

		for i := range updates {
			offset = updates[i].UpdateID + 1
			msg, ok := s.toIncoming(updates[i])
			if !ok {
				continue
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case out <- msg:
			}
		}
		if len(updates) > 0 {
			s.saveOffset(ctx, offset)
		}
	}
}

func (s *Source) getUpdates(ctx context.Context, offset int64) ([]update, error) {
	payload, err := json.Marshal(map[string]any{
		"offset":          offset,
		"timeout":         s.poll,
		"allowed_updates": []string{"message"},
	})
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/bot%s/getUpdates", s.apiBase, s.token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}

	var body struct {
		OK          bool     `json:"ok"`
		Description string   `json:"description"`
		Result      []update `json:"result"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, err
	}
	if !body.OK {
		if body.Description == "" {
			body.Description = fmt.Sprintf("status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("telegram getUpdates: %s", body.Description)
	}
	return body.Result, nil
}

// toIncoming convierte un update en mensaje normalizado. Devuelve false para
// eventos sin texto (o solo con adjuntos) y mensajes de bots. El pipeline
// descarta después a los remitentes que no estén en la lista blanca.
func (s *Source) toIncoming(u update) (domain.IncomingMessage, bool) {
	m := u.Message
	if m == nil || m.From == nil || m.From.IsBot {
		return domain.IncomingMessage{}, false
	}

	content := strings.TrimSpace(m.Text)
	if content == "" {
		content = strings.TrimSpace(m.Caption)
	}
	if content == "" {
		return domain.IncomingMessage{}, false
	}

	received := time.Now().UTC()
	if m.Date > 0 {
		received = time.Unix(m.Date, 0).UTC()
	}

	return domain.IncomingMessage{
		ID:               strconv.FormatInt(u.UpdateID, 10),
		Source:           domain.SourceTelegram,
		ClientIdentifier: strconv.FormatInt(m.From.ID, 10),
		ClientName:       m.From.displayName(),
		RawContent:       content,
		ReceivedAt:       received,
	}, true
}

func (s *Source) loadOffset(ctx context.Context) int64 {
	if s.cursor == nil {
		return 0
	}
	state, err := s.cursor.GetSyncState(ctx, domain.SourceTelegram)
	if errors.Is(err, persistence.ErrNotFound) {
		return 0
	}
	if err != nil {
		s.logger.Error("telegram offset", "error", err)
		return 0
	}
	n, err := strconv.ParseInt(state.Cursor, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func (s *Source) saveOffset(ctx context.Context, offset int64) {
	if s.cursor == nil {
		return
	}
	state, err := s.cursor.GetSyncState(ctx, domain.SourceTelegram)
	if err != nil && !errors.Is(err, persistence.ErrNotFound) {
		s.logger.Error("telegram offset", "error", err)
		return
	}
	if state == nil {
		state = &domain.SyncState{Source: domain.SourceTelegram}
	}
	state.Cursor = strconv.FormatInt(offset, 10)
	if err := s.cursor.SaveSyncState(ctx, state); err != nil && ctx.Err() == nil {
		s.logger.Error("telegram offset", "error", err)
	}
}

func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

type update struct {
	UpdateID int64    `json:"update_id"`
	Message  *message `json:"message"`
}

type message struct {
	MessageID int64  `json:"message_id"`
	From      *user  `json:"from"`
	Date      int64  `json:"date"`
	Text      string `json:"text"`
	Caption   string `json:"caption"`
}

type user struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

func (u *user) displayName() string {
	if name := strings.TrimSpace(u.FirstName + " " + u.LastName); name != "" {
		return name
	}
	if u.Username != "" {
		return "@" + u.Username
	}
	return strconv.FormatInt(u.ID, 10)
}
