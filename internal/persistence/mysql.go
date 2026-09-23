package persistence

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"

	"github.com/nestorPons/ai-assistant/internal/domain"
)

//go:embed migrations/001_init.sql
var initSchema string

// MySQLStore implementa Store sobre MariaDB.
type MySQLStore struct {
	db *sql.DB
}

// NewMySQLStore abre una conexión y aplica las migraciones embebidas.
func NewMySQLStore(ctx context.Context, dsn string) (*MySQLStore, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}
	cfg.MultiStatements = true
	cfg.ParseTime = true

	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}

	s := &MySQLStore{db: db}
	if err := s.migrate(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

// Close cierra la conexión.
func (s *MySQLStore) Close() error { return s.db.Close() }

// migrate aplica el esquema inicial de forma idempotente.
func (s *MySQLStore) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, initSchema)
	return err
}

func (s *MySQLStore) FindBySourceAndIdentifier(ctx context.Context, source domain.Source, identifier string) (*domain.Client, error) {
	const q = `SELECT id, source, identifier, name, tracked, active, created_at, updated_at
	           FROM clients WHERE source = ? AND identifier = ? LIMIT 1`
	c := &domain.Client{}
	err := s.db.QueryRowContext(ctx, q, string(source), identifier).
		Scan(&c.ID, &c.Source, &c.Identifier, &c.Name, &c.Tracked, &c.Active, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (s *MySQLStore) CreateClient(ctx context.Context, client *domain.Client) error {
	if client.ID == "" {
		client.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	client.CreatedAt = now
	client.UpdatedAt = now
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO clients (id, source, identifier, name, tracked, active, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		client.ID, string(client.Source), client.Identifier, client.Name, client.Tracked, client.Active, client.CreatedAt, client.UpdatedAt)
	return mapErr(err)
}

func (s *MySQLStore) UpdateClient(ctx context.Context, client *domain.Client) error {
	client.UpdatedAt = time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`UPDATE clients SET name = ?, tracked = ?, active = ?, updated_at = ? WHERE id = ?`,
		client.Name, client.Tracked, client.Active, client.UpdatedAt, client.ID)
	if err != nil {
		return mapErr(err)
	}
	return ensureAffected(res)
}

func (s *MySQLStore) ListClients(ctx context.Context, trackedOnly bool) ([]domain.Client, error) {
	q := `SELECT id, source, identifier, name, tracked, active, created_at, updated_at FROM clients`
	if trackedOnly {
		q += ` WHERE tracked = TRUE`
	}
	q += ` ORDER BY created_at DESC`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []domain.Client{}
	for rows.Next() {
		var c domain.Client
		if err := rows.Scan(&c.ID, &c.Source, &c.Identifier, &c.Name, &c.Tracked, &c.Active, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *MySQLStore) ExistsBySourceAndExternalID(ctx context.Context, source domain.Source, externalID string) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM raw_messages WHERE source = ? AND external_id = ?)`
	var exists bool
	if err := s.db.QueryRowContext(ctx, q, string(source), externalID).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (s *MySQLStore) CreateRawMessage(ctx context.Context, msg *domain.RawMessage) error {
	if msg.ID == "" {
		msg.ID = uuid.NewString()
	}
	msg.CreatedAt = time.Now().UTC()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO raw_messages (id, client_id, source, external_id, content, processed, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.ClientID, string(msg.Source), msg.ExternalID, msg.Content, msg.Processed, msg.CreatedAt)
	return mapErr(err)
}

func (s *MySQLStore) MarkProcessed(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE raw_messages SET processed = TRUE WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return ensureAffected(res)
}

func (s *MySQLStore) ListRawMessages(ctx context.Context, clientID string) ([]domain.RawMessage, error) {
	q := `SELECT id, client_id, source, external_id, content, processed, created_at FROM raw_messages`
	args := []any{}
	if clientID != "" {
		q += ` WHERE client_id = ?`
		args = append(args, clientID)
	}
	q += ` ORDER BY created_at DESC`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []domain.RawMessage{}
	for rows.Next() {
		var m domain.RawMessage
		if err := rows.Scan(&m.ID, &m.ClientID, &m.Source, &m.ExternalID, &m.Content, &m.Processed, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *MySQLStore) CreateTask(ctx context.Context, task *domain.Task) error {
	if task.ID == "" {
		task.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	task.CreatedAt = now
	task.UpdatedAt = now

	spec, err := domain.MarshalSpecifications(task.Specifications)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO tasks (id, client_id, message_id, title, description, priority, estimated_hours, specifications, status, ai_confidence, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ID, task.ClientID, task.MessageID, task.Title, task.Description, string(task.Priority),
		task.EstimatedHours, spec, string(task.Status), task.AIConfidence, task.CreatedAt, task.UpdatedAt)
	return mapErr(err)
}

func (s *MySQLStore) GetTask(ctx context.Context, id string) (*domain.Task, error) {
	const q = `SELECT id, client_id, message_id, title, description, priority, estimated_hours, specifications, status, ai_confidence, created_at, updated_at
	           FROM tasks WHERE id = ? LIMIT 1`
	t, err := scanTask(s.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (s *MySQLStore) ListTasks(ctx context.Context, status *domain.TaskStatus) ([]domain.Task, error) {
	q := `SELECT id, client_id, message_id, title, description, priority, estimated_hours, specifications, status, ai_confidence, created_at, updated_at FROM tasks`
	args := []any{}
	if status != nil {
		q += ` WHERE status = ?`
		args = append(args, string(*status))
	}
	q += ` ORDER BY created_at DESC`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []domain.Task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

func (s *MySQLStore) UpdateTaskStatus(ctx context.Context, id string, status domain.TaskStatus) error {
	res, err := s.db.ExecContext(ctx, `UPDATE tasks SET status = ?, updated_at = ? WHERE id = ?`, string(status), time.Now().UTC(), id)
	if err != nil {
		return err
	}
	return ensureAffected(res)
}

func (s *MySQLStore) UpdateTask(ctx context.Context, task *domain.Task) error {
	task.UpdatedAt = time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`UPDATE tasks SET title = ?, description = ?, priority = ?, estimated_hours = ?, status = ?, updated_at = ? WHERE id = ?`,
		task.Title, task.Description, string(task.Priority), task.EstimatedHours, string(task.Status), task.UpdatedAt, task.ID)
	if err != nil {
		return err
	}
	return ensureAffected(res)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanTask(row scanner) (*domain.Task, error) {
	t := &domain.Task{}
	var spec []byte
	err := row.Scan(&t.ID, &t.ClientID, &t.MessageID, &t.Title, &t.Description, &t.Priority,
		&t.EstimatedHours, &spec, &t.Status, &t.AIConfidence, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if len(spec) > 0 {
		s, err := domain.UnmarshalSpecifications(spec)
		if err != nil {
			return nil, err
		}
		t.Specifications = s
	}
	return t, nil
}

func ensureAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// mapErr traduce errores del driver a errores de dominio.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	var me *mysql.MySQLError
	if errors.As(err, &me) && me.Number == 1062 {
		return ErrDuplicate
	}
	return err
}
