package anonymizer

import (
	"context"
	"sync"
	"time"
)

// MappingTable asocia cada token con el valor PII original.
type MappingTable map[string]string

// MappingStore define el almacenamiento temporal de la tabla de correspondencias.
type MappingStore interface {
	Save(ctx context.Context, requestID string, mapping MappingTable, ttl time.Duration) error
	Load(ctx context.Context, requestID string) (MappingTable, error)
	Delete(ctx context.Context, requestID string) error
}

// MemoryMappingStore conserva las tablas en RAM con TTL (MVP síncrono).
type MemoryMappingStore struct {
	mu    sync.Mutex
	items map[string]mappingEntry
}

type mappingEntry struct {
	table     MappingTable
	expiresAt time.Time
}

// NewMemoryMappingStore crea un almacén en memoria.
func NewMemoryMappingStore() *MemoryMappingStore {
	return &MemoryMappingStore{items: make(map[string]mappingEntry)}
}

// Save guarda la tabla con TTL.
func (s *MemoryMappingStore) Save(_ context.Context, requestID string, mapping MappingTable, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make(MappingTable, len(mapping))
	for k, v := range mapping {
		cp[k] = v
	}
	s.items[requestID] = mappingEntry{table: cp, expiresAt: time.Now().Add(ttl)}
	return nil
}

// Load recupera la tabla si existe y no ha expirado.
func (s *MemoryMappingStore) Load(_ context.Context, requestID string) (MappingTable, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.items[requestID]
	if !ok {
		return nil, ErrMappingNotFound
	}
	if time.Now().After(e.expiresAt) {
		delete(s.items, requestID)
		return nil, ErrMappingNotFound
	}
	cp := make(MappingTable, len(e.table))
	for k, v := range e.table {
		cp[k] = v
	}
	return cp, nil
}

// Delete elimina la tabla.
func (s *MemoryMappingStore) Delete(_ context.Context, requestID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, requestID)
	return nil
}
