package anonymizer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client es el cliente HTTP de core-engine hacia llm-anonymizer.
type Client struct {
	baseURL string
	token   string
	client  *http.Client
}

// NewClient crea el cliente hacia el microservicio interno.
func NewClient(baseURL, token string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		client:  &http.Client{Timeout: timeout},
	}
}

// Sanitize envía el texto autorizado para ofuscar PII.
func (c *Client) Sanitize(ctx context.Context, req SanitizeRequest) (SanitizeResponse, error) {
	var out SanitizeResponse
	err := c.do(ctx, http.MethodPost, "/api/sanitize", req, &out)
	return out, err
}

// Revert restaura los tokens en el texto procesado.
func (c *Client) Revert(ctx context.Context, req RevertRequest) (RevertResponse, error) {
	var out RevertResponse
	err := c.do(ctx, http.MethodPost, "/api/revert", req, &out)
	return out, err
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	httpReq, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		var er ErrorResponse
		msg := strings.TrimSpace(string(data))
		if json.Unmarshal(data, &er) == nil && er.Error.Message != "" {
			msg = er.Error.Message
		}
		return fmt.Errorf("anonymizer status %d: %s", resp.StatusCode, msg)
	}
	return json.Unmarshal(data, out)
}
