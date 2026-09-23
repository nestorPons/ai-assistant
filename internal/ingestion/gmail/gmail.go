// Package gmail implementa un listener de Gmail mediante la API REST (polling).
// El SDK externo se aísla aquí; el resto del sistema solo ve IncomingMessage.
package gmail

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/nestorPons/ai-assistant/internal/domain"
)

const baseURL = "https://gmail.googleapis.com/gmail/v1"

// Source hace polling de la bandeja de Gmail y emite mensajes no leídos.
type Source struct {
	accessToken string
	query       string
	poll        time.Duration
	client      *http.Client
	lastHistID  string
}

// Config ajusta el listener de Gmail.
type Config struct {
	AccessToken string
	Query       string
	Poll        time.Duration
}

// New crea una fuente Gmail.
func New(cfg Config) *Source {
	if cfg.Poll <= 0 {
		cfg.Poll = 60 * time.Second
	}
	if cfg.Query == "" {
		cfg.Query = "is:unread"
	}
	return &Source{
		accessToken: cfg.AccessToken,
		query:       cfg.Query,
		poll:        cfg.Poll,
		client:      &http.Client{Timeout: 30 * time.Second},
	}
}

// Name identifica el canal.
func (s *Source) Name() string { return "gmail" }

// Start ejecuta el bucle de polling.
func (s *Source) Start(ctx context.Context, out chan<- domain.IncomingMessage) error {
	ticker := time.NewTicker(s.poll)
	defer ticker.Stop()

	for {
		if err := s.pollOnce(ctx, out); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// se registra fuera; aquí se sigue intentando en el siguiente ciclo
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (s *Source) pollOnce(ctx context.Context, out chan<- domain.IncomingMessage) error {
	ids, err := s.listIDs(ctx)
	if err != nil {
		return err
	}
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
	url := fmt.Sprintf("%s/users/me/messages?q=%s&maxResults=25", baseURL, s.query)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+s.accessToken)

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
	url := fmt.Sprintf("%s/users/me/messages/%s?format=metadata&metadataHeaders=From&metadataHeaders=Subject", baseURL, id)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+s.accessToken)

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
		ID      string `json:"id"`
		Snippet string `json:"snippet"`
		Payload struct {
			Headers []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"headers"`
		} `json:"payload"`
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

	content := strings.TrimSpace(subject + "\n" + msg.Snippet)

	return domain.IncomingMessage{
		ID:               msg.ID,
		Source:           domain.SourceGmail,
		ClientIdentifier: email,
		ClientName:       extractName(from),
		RawContent:       content,
		ReceivedAt:       time.Now().UTC(),
	}, nil
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
