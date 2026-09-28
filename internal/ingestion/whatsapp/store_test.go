package whatsapp

import (
	"context"
	"log/slog"
	"path/filepath"
	"testing"
)

// TestStoreSQLite valida que la sesión se inicializa correctamente sobre el
// driver SQLite puro (modernc) y el sqlstore de whatsmeow.
func TestStoreSQLite(t *testing.T) {
	src := New(Config{
		DBPath: filepath.Join(t.TempDir(), "whatsapp.db"),
		Logger: slog.New(slog.DiscardHandler),
	})

	client, err := src.newClient(context.Background())
	if err != nil {
		t.Fatalf("newClient: %v", err)
	}
	defer func() {
		if src.container != nil {
			_ = src.container.Close()
		}
	}()

	if client == nil {
		t.Fatal("cliente nulo")
	}
	if client.Store.ID != nil {
		t.Fatalf("dispositivo nuevo no debería tener ID, got %v", client.Store.ID)
	}

	// Reabrir la misma base y confirmar que el esquema persiste.
	src2 := New(Config{
		DBPath: src.dbPath,
		Logger: slog.New(slog.DiscardHandler),
	})
	client2, err := src2.newClient(context.Background())
	if err != nil {
		t.Fatalf("reapertura: %v", err)
	}
	defer func() {
		if src2.container != nil {
			_ = src2.container.Close()
		}
	}()
	if client2.Store.ID != nil {
		t.Fatalf("dispositivo reabierto no debería tener ID, got %v", client2.Store.ID)
	}
}
