package gmail

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/nestorPons/ai-assistant/internal/domain"
	"github.com/nestorPons/ai-assistant/internal/persistence"
)

// errHistoryExpired indica que el historyId guardado ya no es válido en Gmail.
var errHistoryExpired = errors.New("gmail: historial expirado")

// Sync sincroniza los mensajes nuevos usando la History API y actualiza el
// cursor. Serializado para que el worker de Pub/Sub y el ticker de respaldo no
// pisen el cursor a la vez.
func (s *Source) Sync(ctx context.Context, out chan<- domain.IncomingMessage) error {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()

	if s.cursor == nil {
		return errors.New("gmail: sin cursor store")
	}

	state, err := s.cursor.GetSyncState(ctx, domain.SourceGmail)
	if err != nil && !errors.Is(err, persistence.ErrNotFound) {
		return err
	}

	// Primer arranque: solo baseline, sin procesar el historial previo.
	if state == nil || state.Cursor == "" {
		if s.backlog {
			s.logger.Info("gmail baseline con backlog inicial")
			return s.resync(ctx, out)
		}
		historyID, err := s.getProfile(ctx)
		if err != nil {
			return err
		}
		s.logger.Info("gmail baseline establecido", "history_id", historyID)
		return s.saveCursor(ctx, historyID, time.Time{})
	}

	ids, newHistoryID, err := s.historyList(ctx, state.Cursor)
	if errors.Is(err, errHistoryExpired) {
		s.logger.Warn("gmail historial expirado, resincronizando")
		return s.resync(ctx, out)
	}
	if err != nil {
		return err
	}

	if err := s.emit(ctx, out, ids); err != nil {
		return err
	}
	if newHistoryID != "" {
		return s.saveCursor(ctx, newHistoryID, time.Time{})
	}
	return nil
}

// resync recupera mensajes tras expirar el historial y restablece el baseline.
func (s *Source) resync(ctx context.Context, out chan<- domain.IncomingMessage) error {
	ids, err := s.listIDs(ctx)
	if err != nil {
		return err
	}
	if err := s.emit(ctx, out, ids); err != nil {
		return err
	}
	historyID, err := s.getProfile(ctx)
	if err != nil {
		return err
	}
	return s.saveCursor(ctx, historyID, time.Time{})
}

func (s *Source) getProfile(ctx context.Context) (string, error) {
	token, err := s.authToken(ctx)
	if err != nil {
		return "", err
	}

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"/users/me/profile", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gmail profile status %d", resp.StatusCode)
	}

	var body struct {
		HistoryID string `json:"historyId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	if body.HistoryID == "" {
		return "", errors.New("gmail profile sin historyId")
	}
	return body.HistoryID, nil
}

func (s *Source) historyList(ctx context.Context, startHistoryID string) ([]string, string, error) {
	token, err := s.authToken(ctx)
	if err != nil {
		return nil, "", err
	}

	endpoint := fmt.Sprintf("%s/users/me/history?startHistoryId=%s&historyTypes=messageAdded&maxResults=100",
		s.baseURL, url.QueryEscape(startHistoryID))

	var (
		ids     []string
		latest  string
		pageTok string
	)
	for {
		u := endpoint
		if pageTok != "" {
			u += "&pageToken=" + url.QueryEscape(pageTok)
		}
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := s.client.Do(req)
		if err != nil {
			return nil, "", err
		}
		if resp.StatusCode == http.StatusNotFound {
			resp.Body.Close()
			return nil, "", errHistoryExpired
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, "", fmt.Errorf("gmail history status %d", resp.StatusCode)
		}

		data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if err != nil {
			return nil, "", err
		}

		var body struct {
			History []struct {
				MessagesAdded []struct {
					Message struct {
						ID string `json:"id"`
					} `json:"message"`
				} `json:"messagesAdded"`
			} `json:"history"`
			NextPageToken string `json:"nextPageToken"`
			HistoryID     string `json:"historyId"`
		}
		if err := json.Unmarshal(data, &body); err != nil {
			return nil, "", err
		}

		for _, h := range body.History {
			for _, m := range h.MessagesAdded {
				if m.Message.ID != "" {
					ids = append(ids, m.Message.ID)
				}
			}
		}
		if body.HistoryID != "" {
			latest = body.HistoryID
		}
		if body.NextPageToken == "" {
			break
		}
		pageTok = body.NextPageToken
	}
	return ids, latest, nil
}

func (s *Source) saveCursor(ctx context.Context, cursor string, expires time.Time) error {
	state, err := s.cursor.GetSyncState(ctx, domain.SourceGmail)
	if err != nil && !errors.Is(err, persistence.ErrNotFound) {
		return err
	}
	if state == nil {
		state = &domain.SyncState{Source: domain.SourceGmail}
	}
	state.Cursor = cursor
	if !expires.IsZero() {
		state.ExpiresAt = expires
	}
	return s.cursor.SaveSyncState(ctx, state)
}
