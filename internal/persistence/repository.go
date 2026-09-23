// Package persistence define los repositorios tipados y las implementaciones
// SQL (MariaDB) y en memoria (pruebas). El acceso al contenido original está
// restringido a esta capa y nunca se expone en logs ni a proveedores externos.
package persistence

import (
	"context"
	"errors"

	"github.com/nestorPons/ai-assistant/internal/domain"
)

// ErrNotFound indica que un registro no existe.
var ErrNotFound = errors.New("registro no encontrado")

// ErrDuplicate indica una violación de unicidad (p. ej. mensaje duplicado).
var ErrDuplicate = errors.New("registro duplicado")

// Store agrupa los repositorios del sistema.
type Store interface {
	ClientRepository
	RawMessageRepository
	TaskRepository
}

// ClientRepository gestiona la lista blanca de contactos autorizados.
type ClientRepository interface {
	// FindBySourceAndIdentifier localiza un contacto por canal e identificador.
	FindBySourceAndIdentifier(ctx context.Context, source domain.Source, identifier string) (*domain.Client, error)
	// CreateClient da de alta un contacto (exige tracked/active explícitos).
	CreateClient(ctx context.Context, client *domain.Client) error
	// UpdateClient modifica nombre, tracked y active.
	UpdateClient(ctx context.Context, client *domain.Client) error
	// ListClients devuelve los contactos, opcionalmente solo los rastreados.
	ListClients(ctx context.Context, trackedOnly bool) ([]domain.Client, error)
}

// RawMessageRepository gestiona el registro auditable de mensajes.
type RawMessageRepository interface {
	// ExistsBySourceAndExternalID comprueba idempotencia.
	ExistsBySourceAndExternalID(ctx context.Context, source domain.Source, externalID string) (bool, error)
	// CreateRawMessage inserta el mensaje original autorizado.
	CreateRawMessage(ctx context.Context, msg *domain.RawMessage) error
	// MarkProcessed marca el mensaje como procesado.
	MarkProcessed(ctx context.Context, id string) error
	// ListRawMessages devuelve mensajes (acceso restringido, uso interno autorizado).
	ListRawMessages(ctx context.Context, clientID string) ([]domain.RawMessage, error)
}

// TaskRepository gestiona las tareas procesadas.
type TaskRepository interface {
	CreateTask(ctx context.Context, task *domain.Task) error
	GetTask(ctx context.Context, id string) (*domain.Task, error)
	ListTasks(ctx context.Context, status *domain.TaskStatus) ([]domain.Task, error)
	UpdateTaskStatus(ctx context.Context, id string, status domain.TaskStatus) error
	UpdateTask(ctx context.Context, task *domain.Task) error
}
