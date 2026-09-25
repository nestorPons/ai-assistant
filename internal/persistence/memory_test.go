package persistence

import (
	"context"
	"testing"

	"github.com/nestorPons/ai-assistant/internal/domain"
)

func TestMemoryStoreClientLifecycle(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	c := &domain.Client{Source: domain.SourceGmail, Identifier: "maria@example.com", Name: "María", Tracked: true, Active: true}
	if err := s.CreateClient(ctx, c); err != nil {
		t.Fatal(err)
	}

	found, err := s.FindBySourceAndIdentifier(ctx, domain.SourceGmail, "maria@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if !found.Authorized() {
		t.Error("cliente debería estar autorizado")
	}

	// Duplicado rechazado.
	if err := s.CreateClient(ctx, &domain.Client{Source: domain.SourceGmail, Identifier: "maria@example.com"}); err != ErrDuplicate {
		t.Errorf("esperaba ErrDuplicate, fue %v", err)
	}

	// Desactivar.
	found.Active = false
	if err := s.UpdateClient(ctx, found); err != nil {
		t.Fatal(err)
	}
	found, _ = s.FindBySourceAndIdentifier(ctx, domain.SourceGmail, "maria@example.com")
	if found.Authorized() {
		t.Error("cliente inactivo no debe estar autorizado")
	}
}

func TestMemoryStoreRawMessageIdempotency(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	m := &domain.RawMessage{ClientID: "c1", Source: domain.SourceGmail, ExternalID: "ext1", Content: "hola"}
	if err := s.CreateRawMessage(ctx, m); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateRawMessage(ctx, &domain.RawMessage{ClientID: "c1", Source: domain.SourceGmail, ExternalID: "ext1", Content: "hola"}); err != ErrDuplicate {
		t.Errorf("esperaba ErrDuplicate, fue %v", err)
	}

	exists, _ := s.ExistsBySourceAndExternalID(ctx, domain.SourceGmail, "ext1")
	if !exists {
		t.Error("debería existir")
	}

	if err := s.MarkProcessed(ctx, m.ID); err != nil {
		t.Fatal(err)
	}
}

func TestMemoryStoreTasks(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	task := &domain.Task{ClientID: "c1", MessageID: "m1", Title: "t", Description: "d", Subject: "nestorpons.com", Priority: domain.PriorityMedium, Status: domain.TaskStatusPending}
	if err := s.CreateTask(ctx, task); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "t" {
		t.Error("título incorrecto")
	}
	if got.Subject != "nestorpons.com" {
		t.Errorf("tema = %q", got.Subject)
	}
	if got.DueDate != nil {
		t.Errorf("fecha límite debería ser nil, fue %v", got.DueDate)
	}

	pending := domain.TaskStatusPending
	list, _ := s.ListTasks(ctx, &pending)
	if len(list) != 1 {
		t.Errorf("esperaba 1 tarea, hay %d", len(list))
	}

	if err := s.UpdateTaskStatus(ctx, task.ID, domain.TaskStatusCompleted); err != nil {
		t.Fatal(err)
	}
}

func TestMemoryStoreSyncState(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	if _, err := s.GetSyncState(ctx, domain.SourceGmail); err != ErrNotFound {
		t.Fatalf("esperaba ErrNotFound, fue %v", err)
	}

	if err := s.SaveSyncState(ctx, &domain.SyncState{Source: domain.SourceGmail, Cursor: "100"}); err != nil {
		t.Fatal(err)
	}
	st, err := s.GetSyncState(ctx, domain.SourceGmail)
	if err != nil {
		t.Fatal(err)
	}
	if st.Cursor != "100" {
		t.Errorf("cursor = %q, want 100", st.Cursor)
	}

	if err := s.SaveSyncState(ctx, &domain.SyncState{Source: domain.SourceGmail, Cursor: "200"}); err != nil {
		t.Fatal(err)
	}
	st, _ = s.GetSyncState(ctx, domain.SourceGmail)
	if st.Cursor != "200" {
		t.Errorf("cursor = %q, want 200", st.Cursor)
	}
}
