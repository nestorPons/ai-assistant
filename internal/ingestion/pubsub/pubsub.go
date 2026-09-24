// Package pubsub implementa un lector pull de Google Cloud Pub/Sub para recibir
// notificaciones de Gmail y disparar la sincronización incremental. No expone
// ningún endpoint público.
package pubsub

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/nestorPons/ai-assistant/internal/oauth"
)

const (
	defaultBaseURL = "https://pubsub.googleapis.com/v1"
	defaultMaxMsgs = 10
	retryBackoff   = 5 * time.Second
)

// Notification es el payload que Gmail publica en el topic.
type Notification struct {
	EmailAddress string `json:"emailAddress"`
	HistoryID    string `json:"historyId"`
}

// ReceivedMessage es un mensaje de Pub/Sub ya decodificado.
type ReceivedMessage struct {
	AckID        string
	Notification Notification
}

// Client llama a la REST API de Pub/Sub con un TokenSource compartido.
type Client struct {
	tokens            *oauth.TokenSource
	baseURL           string
	client            *http.Client
	returnImmediately bool
}

// New crea un cliente de Pub/Sub.
func New(tokens *oauth.TokenSource) *Client {
	return &Client{
		tokens:  tokens,
		baseURL: defaultBaseURL,
		client:  &http.Client{Timeout: 90 * time.Second},
	}
}

// Run lee la suscripción y ejecuta handler por cada lote recibido. Hace ack
// solo si handler termina bien; ante error deja los mensajes para reentrega
// (la idempotencia del pipeline evita duplicados).
func (c *Client) Run(ctx context.Context, subscription string, handler func(context.Context) error) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		msgs, err := c.Pull(ctx, subscription, defaultMaxMsgs)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err := sleep(ctx, retryBackoff); err != nil {
				return err
			}
			continue
		}
		if len(msgs) == 0 {
			continue
		}

		ackIDs := make([]string, 0, len(msgs))
		for _, m := range msgs {
			ackIDs = append(ackIDs, m.AckID)
		}

		if err := handler(ctx); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err := sleep(ctx, retryBackoff); err != nil {
				return err
			}
			continue
		}

		if err := c.Ack(ctx, subscription, ackIDs); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err := sleep(ctx, time.Second); err != nil {
				return err
			}
		}
	}
}

type receivedMessage struct {
	AckID   string `json:"ackId"`
	Message struct {
		Data string `json:"data"`
	} `json:"message"`
}

// Pull lee hasta max mensajes de la suscripción.
func (c *Client) Pull(ctx context.Context, subscription string, max int) ([]ReceivedMessage, error) {
	token, err := c.tokens.Token(ctx)
	if err != nil {
		return nil, err
	}

	body, _ := json.Marshal(map[string]any{"maxMessages": max, "returnImmediately": c.returnImmediately})
	endpoint := fmt.Sprintf("%s/%s:pull", c.baseURL, strings.TrimPrefix(subscription, "/"))
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pubsub pull status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	var out struct {
		ReceivedMessages []receivedMessage `json:"receivedMessages"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}

	msgs := make([]ReceivedMessage, 0, len(out.ReceivedMessages))
	for _, rm := range out.ReceivedMessages {
		var n Notification
		if raw, err := base64.StdEncoding.DecodeString(rm.Message.Data); err == nil {
			_ = json.Unmarshal(raw, &n)
		}
		msgs = append(msgs, ReceivedMessage{AckID: rm.AckID, Notification: n})
	}
	return msgs, nil
}

// PullOnce hace un pull no bloqueante (returnImmediately) y devuelve lo que
// haya disponible en ese momento. Útil para pruebas y diagnósticos.
func (c *Client) PullOnce(ctx context.Context, subscription string, max int) ([]ReceivedMessage, error) {
	prev := c.returnImmediately
	c.returnImmediately = true
	defer func() { c.returnImmediately = prev }()
	return c.Pull(ctx, subscription, max)
}

// Ack confirma los mensajes procesados.
func (c *Client) Ack(ctx context.Context, subscription string, ackIDs []string) error {
	if len(ackIDs) == 0 {
		return nil
	}

	token, err := c.tokens.Token(ctx)
	if err != nil {
		return err
	}

	body, _ := json.Marshal(map[string]any{"ackIds": ackIDs})
	endpoint := fmt.Sprintf("%s/%s:ack", c.baseURL, strings.TrimPrefix(subscription, "/"))
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pubsub ack status %d", resp.StatusCode)
	}
	return nil
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
