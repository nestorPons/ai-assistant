package persistence

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/nestorPons/ai-assistant/internal/domain"
)

// MemoryStore implementa Store en memoria (para pruebas y arranque sin DB).
type MemoryStore struct {
	mu sync.RWMutex

	clients     map[string]*domain.Client // key: source|identifier
	clientsByID map[string]*domain.Client
	rawMessages map[string]*domain.RawMessage // key: id
	tasks       map[string]*domain.Task       // key: id
	seen        map[string]bool               // key: source|external_id
	syncStates  map[string]*domain.SyncState  // key: source
}

// NewMemoryStore crea un almacén en memoria vacío.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		clients:     make(map[string]*domain.Client),
		clientsByID: make(map[string]*domain.Client),
		rawMessages: make(map[string]*domain.RawMessage),
		tasks:       make(map[string]*domain.Task),
		seen:        make(map[string]bool),
		syncStates:  make(map[string]*domain.SyncState),
	}
}

func clientKey(source domain.Source, identifier string) string {
	return string(source) + "|" + identifier
}

func (s *MemoryStore) FindBySourceAndIdentifier(_ context.Context, source domain.Source, identifier string) (*domain.Client, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.clients[clientKey(source, identifier)]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *c
	return &cp, nil
}

func (s *MemoryStore) CreateClient(_ context.Context, client *domain.Client) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if client.ID == "" {
		client.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	client.CreatedAt = now
	client.UpdatedAt = now
	if _, ok := s.clients[clientKey(client.Source, client.Identifier)]; ok {
		return ErrDuplicate
	}
	cp := *client
	s.clients[clientKey(client.Source, client.Identifier)] = &cp
	s.clientsByID[client.ID] = &cp
	return nil
}

func (s *MemoryStore) UpdateClient(_ context.Context, client *domain.Client) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.clientsByID[client.ID]
	if !ok {
		return ErrNotFound
	}
	client.UpdatedAt = time.Now().UTC()
	c.Name = client.Name
	c.Tracked = client.Tracked
	c.Active = client.Active
	c.UpdatedAt = client.UpdatedAt
	return nil
}

func (s *MemoryStore) ListClients(_ context.Context, trackedOnly bool) ([]domain.Client, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Client, 0, len(s.clients))
	for _, c := range s.clients {
		if trackedOnly && !c.Tracked {
			continue
		}
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (s *MemoryStore) ExistsBySourceAndExternalID(_ context.Context, source domain.Source, externalID string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.seen[string(source)+"|"+externalID], nil
}

func (s *MemoryStore) CreateRawMessage(_ context.Context, msg *domain.RawMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if msg.ID == "" {
		msg.ID = uuid.NewString()
	}
	key := string(msg.Source) + "|" + msg.ExternalID
	if s.seen[key] {
		return ErrDuplicate
	}
	msg.CreatedAt = time.Now().UTC()
	cp := *msg
	s.rawMessages[msg.ID] = &cp
	s.seen[key] = true
	return nil
}

func (s *MemoryStore) MarkProcessed(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.rawMessages[id]
	if !ok {
		return ErrNotFound
	}
	m.Processed = true
	return nil
}

func (s *MemoryStore) ListRawMessages(_ context.Context, clientID string) ([]domain.RawMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.RawMessage, 0, len(s.rawMessages))
	for _, m := range s.rawMessages {
		if clientID != "" && m.ClientID != clientID {
			continue
		}
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (s *MemoryStore) CreateTask(_ context.Context, task *domain.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if task.ID == "" {
		task.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	task.CreatedAt = now
	task.UpdatedAt = now
	cp := *task
	s.tasks[task.ID] = &cp
	return nil
}

func (s *MemoryStore) GetTask(_ context.Context, id string) (*domain.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tasks[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *t
	return &cp, nil
}

func (s *MemoryStore) ListTasks(_ context.Context, status *domain.TaskStatus) ([]domain.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		if status != nil && t.Status != *status {
			continue
		}
		out = append(out, *t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (s *MemoryStore) UpdateTaskStatus(_ context.Context, id string, status domain.TaskStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[id]
	if !ok {
		return ErrNotFound
	}
	t.Status = status
	t.UpdatedAt = time.Now().UTC()
	return nil
}

func (s *MemoryStore) UpdateTask(_ context.Context, task *domain.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[task.ID]
	if !ok {
		return ErrNotFound
	}
	t.Title = task.Title
	t.Description = task.Description
	t.Subject = task.Subject
	t.Priority = task.Priority
	t.EstimatedHours = task.EstimatedHours
	t.DueDate = task.DueDate
	t.Status = task.Status
	t.UpdatedAt = time.Now().UTC()
	return nil
}

func (s *MemoryStore) GetSyncState(_ context.Context, source domain.Source) (*domain.SyncState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st, ok := s.syncStates[string(source)]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *st
	return &cp, nil
}

func (s *MemoryStore) SaveSyncState(_ context.Context, state *domain.SyncState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state.UpdatedAt = time.Now().UTC()
	cp := *state
	s.syncStates[string(state.Source)] = &cp
	return nil
}
