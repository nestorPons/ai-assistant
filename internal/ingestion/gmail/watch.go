package gmail

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nestorPons/ai-assistant/internal/domain"
	"github.com/nestorPons/ai-assistant/internal/persistence"
)

type watchResponse struct {
	HistoryID  string `json:"historyId"`
	Expiration string `json:"expiration"`
}

// ensureWatch registra el watch de Gmail hacia el topic de Pub/Sub. Si no hay
// cursor guardado, usa el historyId devuelto como baseline; siempre actualiza
// la expiración del watch.
func (s *Source) ensureWatch(ctx context.Context) error {
	if s.topicName == "" || s.cursor == nil {
		return nil
	}

	token, err := s.authToken(ctx)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(map[string]any{"topicName": s.topicName})
	if err != nil {
		return err
	}

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/users/me/watch", strings.NewReader(string(payload)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gmail watch status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	var wr watchResponse
	if err := json.Unmarshal(data, &wr); err != nil {
		return err
	}

	var expires time.Time
	if ms, err := strconv.ParseInt(wr.Expiration, 10, 64); err == nil {
		expires = time.UnixMilli(ms)
	}

	state, err := s.cursor.GetSyncState(ctx, domain.SourceGmail)
	if err != nil && !errors.Is(err, persistence.ErrNotFound) {
		return err
	}
	if state == nil || state.Cursor == "" {
		state = &domain.SyncState{Source: domain.SourceGmail, Cursor: wr.HistoryID}
	}
	state.ExpiresAt = expires
	return s.cursor.SaveSyncState(ctx, state)
}
