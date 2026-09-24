package dashboard

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nestorPons/ai-assistant/internal/domain"
	"github.com/nestorPons/ai-assistant/internal/persistence"
)

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("s3cr3t!")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(hash, "pbkdf2_sha256$") {
		t.Fatalf("formato inesperado: %q", hash)
	}
	if !VerifyPassword(hash, "s3cr3t!") {
		t.Fatal("la contraseña correcta debería validar")
	}
	if VerifyPassword(hash, "otra") {
		t.Fatal("una contraseña incorrecta no debería validar")
	}
	if VerifyPassword("basura", "s3cr3t!") {
		t.Fatal("un hash inválido no debería validar")
	}
}

func TestAuthVerify(t *testing.T) {
	hash, _ := HashPassword("clave")
	a := NewAuth("nestor", hash, "")
	if !a.Configured() {
		t.Fatal("debería estar configurado")
	}
	if !a.Verify("nestor", "clave") {
		t.Fatal("credenciales correctas")
	}
	if a.Verify("otro", "clave") || a.Verify("nestor", "mala") {
		t.Fatal("credenciales incorrectas no deben validar")
	}

	open := NewAuth("", "", "")
	if open.User != "admin" {
		t.Fatalf("usuario por defecto: %q", open.User)
	}
	if open.Configured() {
		t.Fatal("sin hash ni contraseña no está configurado")
	}
	if !open.Verify("admin", "") {
		t.Fatal("acceso abierto en desarrollo")
	}
}

func TestEnvFilePreservesComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	initial := "# comentario\nOPENAI_API_KEY=abc\n\nCLASSIFIER_MODE=chain # inline\n"
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}

	env, err := LoadEnvFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := env.Get("OPENAI_API_KEY"); !ok || v != "abc" {
		t.Fatalf("get: %q %v", v, ok)
	}
	env.Set("OPENAI_API_KEY", "nueva clave con espacios")
	env.Set("NUEVA", "1")

	if err := env.Save(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "# comentario") {
		t.Fatal("se perdió el comentario")
	}
	if !strings.Contains(content, `OPENAI_API_KEY="nueva clave con espacios"`) {
		t.Fatalf("no se actualizó la clave:\n%s", content)
	}
	if !strings.Contains(content, "NUEVA=1") {
		t.Fatal("no se añadió la clave nueva")
	}

	reloaded, _ := LoadEnvFile(path)
	if v, _ := reloaded.Get("OPENAI_API_KEY"); v != "nueva clave con espacios" {
		t.Fatalf("recarga: %q", v)
	}
}

func TestViewsSmoke(t *testing.T) {
	store := persistence.NewMemoryStore()
	ctx := context.Background()

	client := &domain.Client{Source: domain.SourceGmail, Identifier: "a@b.com", Name: "Ana", Tracked: true, Active: true}
	if err := store.CreateClient(ctx, client); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateRawMessage(ctx, &domain.RawMessage{ClientID: client.ID, Source: domain.SourceGmail, ExternalID: "m1", Content: "hola mundo"}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateTask(ctx, &domain.Task{ClientID: client.ID, MessageID: "m1", Title: "Tarea demo", Priority: domain.PriorityHigh, Status: domain.TaskStatusPending, AIConfidence: 0.8, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	_ = os.WriteFile(envPath, []byte("OPENAI_API_KEY=abc\n"), 0o600)

	d := New(Options{Store: store, EnvPath: envPath, Auth: NewAuth("admin", "", ""), Backend: "memoria"})

	for name, v := range map[string]view{
		"home":     newHomeView(d),
		"tasks":    newTasksView(d),
		"clients":  newClientsView(d),
		"messages": newMessagesView(d),
		"config":   newConfigView(d),
		"logs":     newLogsView(d),
	} {
		if v.Primitive() == nil {
			t.Fatalf("%s: primitive nil", name)
		}
		v.Refresh()
	}
}
