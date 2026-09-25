// Package domain define las estructuras y tipos compartidos del sistema.
// No debe depender de SDK externos ni de detalles de proveedores.
package domain

import (
	"encoding/json"
	"time"
)

// Source identifica el canal de origen de un mensaje.
type Source string

const (
	SourceWhatsApp Source = "whatsapp"
	SourceTelegram Source = "telegram"
	SourceGmail    Source = "gmail"
)

// Valid devuelve true si el canal está soportado.
func (s Source) Valid() bool {
	switch s {
	case SourceWhatsApp, SourceTelegram, SourceGmail:
		return true
	default:
		return false
	}
}

// IncomingMessage es la estructura interna normalizada de cualquier evento entrante.
type IncomingMessage struct {
	ID               string    `json:"id"`
	Source           Source    `json:"source"`
	ClientIdentifier string    `json:"client_identifier"`
	ClientName       string    `json:"client_name"`
	RawContent       string    `json:"raw_content"`
	ReceivedAt       time.Time `json:"received_at"`
}

// SyncState guarda el cursor de sincronización incremental de un canal.
type SyncState struct {
	Source    Source    `json:"source"`
	Cursor    string    `json:"cursor"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Client representa un contacto autorizado (lista blanca por canal e identificador).
type Client struct {
	ID         string    `json:"id"`
	Source     Source    `json:"source"`
	Identifier string    `json:"identifier"`
	Name       string    `json:"name"`
	Tracked    bool      `json:"tracked"`
	Active     bool      `json:"active"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Authorized devuelve true si el contacto está en la lista blanca y activo.
func (c Client) Authorized() bool {
	return c.Tracked && c.Active
}

// RawMessage es el registro auditable del contenido original autorizado.
type RawMessage struct {
	ID         string    `json:"id"`
	ClientID   string    `json:"client_id"`
	Source     Source    `json:"source"`
	ExternalID string    `json:"external_id"`
	Content    string    `json:"content"`
	Processed  bool      `json:"processed"`
	CreatedAt  time.Time `json:"created_at"`
}

// Priority define el nivel de prioridad de una tarea.
type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
)

func (p Priority) Valid() bool {
	switch p {
	case PriorityLow, PriorityMedium, PriorityHigh:
		return true
	default:
		return false
	}
}

// TaskStatus define el estado de una tarea.
type TaskStatus string

const (
	TaskStatusPending     TaskStatus = "pending"
	TaskStatusInProgress  TaskStatus = "in_progress"
	TaskStatusCompleted   TaskStatus = "completed"
	TaskStatusDiscarded   TaskStatus = "discarded"
	TaskStatusNeedsReview TaskStatus = "needs_review"
)

func (s TaskStatus) Valid() bool {
	switch s {
	case TaskStatusPending, TaskStatusInProgress, TaskStatusCompleted, TaskStatusDiscarded, TaskStatusNeedsReview:
		return true
	default:
		return false
	}
}

// Specifications conserva los requisitos operativos extraídos de una tarea.
type Specifications struct {
	Requirements       []string `json:"requirements,omitempty"`
	Constraints        []string `json:"constraints,omitempty"`
	Deliverables       []string `json:"deliverables,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`
	Dependencies       []string `json:"dependencies,omitempty"`
	OpenQuestions      []string `json:"open_questions,omitempty"`
}

// MarshalSpecifications serializa las especificaciones a JSON (nil si vacías).
func MarshalSpecifications(s *Specifications) ([]byte, error) {
	if s == nil || s.isEmpty() {
		return nil, nil
	}
	return json.Marshal(s)
}

func (s Specifications) isEmpty() bool {
	return len(s.Requirements) == 0 &&
		len(s.Constraints) == 0 &&
		len(s.Deliverables) == 0 &&
		len(s.AcceptanceCriteria) == 0 &&
		len(s.Dependencies) == 0 &&
		len(s.OpenQuestions) == 0
}

// UnmarshalSpecifications deserializa JSON en Specifications.
func UnmarshalSpecifications(b []byte) (*Specifications, error) {
	if len(b) == 0 {
		return nil, nil
	}
	var s Specifications
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// Task representa una tarea procesada y validada por el sistema.
type Task struct {
	ID             string          `json:"id"`
	ClientID       string          `json:"client_id"`
	MessageID      string          `json:"message_id"`
	Title          string          `json:"title"`
	Description    string          `json:"description"`
	Subject        string          `json:"subject"`
	Priority       Priority        `json:"priority"`
	EstimatedHours *float64        `json:"estimated_hours,omitempty"`
	DueDate        *time.Time      `json:"due_date,omitempty"`
	Specifications *Specifications `json:"specifications,omitempty"`
	Status         TaskStatus      `json:"status"`
	AIConfidence   float64         `json:"ai_confidence"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}
