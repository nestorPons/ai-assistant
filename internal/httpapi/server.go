// Package httpapi expone una API mínima para gestionar la lista blanca y
// consultar tareas y mensajes. El dashboard queda desacoplado; esta API sirve
// para probar el sistema.
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/nestorPons/ai-assistant/internal/domain"
	"github.com/nestorPons/ai-assistant/internal/persistence"
)

// Server es el servidor HTTP mínimo.
type Server struct {
	store  persistence.Store
	logger *slog.Logger
}

// New crea el servidor.
func New(store persistence.Store, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{store: store, logger: logger}
}

// Handler devuelve el router.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /clients", s.listClients)
	mux.HandleFunc("POST /clients", s.createClient)
	mux.HandleFunc("PUT /clients/{id}", s.updateClient)

	mux.HandleFunc("GET /tasks", s.listTasks)
	mux.HandleFunc("GET /tasks/{id}", s.getTask)
	mux.HandleFunc("PATCH /tasks/{id}", s.patchTask)
	mux.HandleFunc("PUT /tasks/{id}", s.updateTask)

	mux.HandleFunc("GET /messages", s.listMessages)

	return mux
}

func (s *Server) listClients(w http.ResponseWriter, r *http.Request) {
	trackedOnly := r.URL.Query().Get("tracked") == "true"
	clients, err := s.store.ListClients(r.Context(), trackedOnly)
	if err != nil {
		s.error(w, http.StatusInternalServerError, err)
		return
	}
	s.write(w, http.StatusOK, clients)
}

func (s *Server) createClient(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Source     string `json:"source"`
		Identifier string `json:"identifier"`
		Name       string `json:"name"`
		Tracked    *bool  `json:"tracked"`
		Active     *bool  `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.error(w, http.StatusBadRequest, err)
		return
	}
	tracked, active := false, true
	if req.Tracked != nil {
		tracked = *req.Tracked
	}
	if req.Active != nil {
		active = *req.Active
	}
	client := &domain.Client{
		Source:     domain.Source(req.Source),
		Identifier: req.Identifier,
		Name:       req.Name,
		Tracked:    tracked,
		Active:     active,
	}
	if !client.Source.Valid() || client.Identifier == "" {
		s.error(w, http.StatusBadRequest, errors.New("source o identifier inválidos"))
		return
	}
	if err := s.store.CreateClient(r.Context(), client); err != nil {
		s.error(w, http.StatusInternalServerError, err)
		return
	}
	s.write(w, http.StatusCreated, client)
}

func (s *Server) updateClient(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Name    *string `json:"name"`
		Tracked *bool   `json:"tracked"`
		Active  *bool   `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.error(w, http.StatusBadRequest, err)
		return
	}
	// Se recorre la lista para localizar por id; en producción se añadiría GetClient.
	clients, err := s.store.ListClients(r.Context(), false)
	if err != nil {
		s.error(w, http.StatusInternalServerError, err)
		return
	}
	var target *domain.Client
	for i := range clients {
		if clients[i].ID == id {
			target = &clients[i]
			break
		}
	}
	if target == nil {
		s.error(w, http.StatusNotFound, persistence.ErrNotFound)
		return
	}
	if req.Name != nil {
		target.Name = *req.Name
	}
	if req.Tracked != nil {
		target.Tracked = *req.Tracked
	}
	if req.Active != nil {
		target.Active = *req.Active
	}
	if err := s.store.UpdateClient(r.Context(), target); err != nil {
		s.error(w, http.StatusInternalServerError, err)
		return
	}
	s.write(w, http.StatusOK, target)
}

func (s *Server) listTasks(w http.ResponseWriter, r *http.Request) {
	var status *domain.TaskStatus
	if v := r.URL.Query().Get("status"); v != "" {
		st := domain.TaskStatus(v)
		status = &st
	}
	tasks, err := s.store.ListTasks(r.Context(), status)
	if err != nil {
		s.error(w, http.StatusInternalServerError, err)
		return
	}
	s.write(w, http.StatusOK, tasks)
}

func (s *Server) getTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	task, err := s.store.GetTask(r.Context(), id)
	if errors.Is(err, persistence.ErrNotFound) {
		s.error(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		s.error(w, http.StatusInternalServerError, err)
		return
	}
	s.write(w, http.StatusOK, task)
}

func (s *Server) patchTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.error(w, http.StatusBadRequest, err)
		return
	}
	status := domain.TaskStatus(req.Status)
	if !status.Valid() {
		s.error(w, http.StatusBadRequest, errors.New("estado inválido"))
		return
	}
	if err := s.store.UpdateTaskStatus(r.Context(), id, status); err != nil {
		s.error(w, http.StatusInternalServerError, err)
		return
	}
	s.write(w, http.StatusOK, map[string]string{"id": id, "status": string(status)})
}

func (s *Server) updateTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	task, err := s.store.GetTask(r.Context(), id)
	if errors.Is(err, persistence.ErrNotFound) {
		s.error(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		s.error(w, http.StatusInternalServerError, err)
		return
	}
	var req struct {
		Title       *string `json:"title"`
		Description *string `json:"description"`
		Subject     *string `json:"subject"`
		Priority    *string `json:"priority"`
		Status      *string `json:"status"`
		DueDate     *string `json:"due_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.error(w, http.StatusBadRequest, err)
		return
	}
	if req.Title != nil {
		task.Title = *req.Title
	}
	if req.Description != nil {
		task.Description = *req.Description
	}
	if req.Subject != nil {
		task.Subject = *req.Subject
	}
	if req.Priority != nil {
		task.Priority = domain.Priority(*req.Priority)
	}
	if req.Status != nil {
		task.Status = domain.TaskStatus(*req.Status)
	}
	if req.DueDate != nil {
		due, err := parseDueDate(*req.DueDate)
		if err != nil {
			s.error(w, http.StatusBadRequest, err)
			return
		}
		task.DueDate = due
	}
	if err := s.store.UpdateTask(r.Context(), task); err != nil {
		s.error(w, http.StatusInternalServerError, err)
		return
	}
	s.write(w, http.StatusOK, task)
}

func (s *Server) listMessages(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")
	msgs, err := s.store.ListRawMessages(r.Context(), clientID)
	if err != nil {
		s.error(w, http.StatusInternalServerError, err)
		return
	}
	s.write(w, http.StatusOK, msgs)
}

func (s *Server) error(w http.ResponseWriter, status int, err error) {
	s.write(w, status, map[string]string{"error": err.Error()})
}

func (s *Server) write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// parseDueDate interpreta una fecha YYYY-MM-DD; cadena vacía limpia el valor.
func parseDueDate(raw string) (*time.Time, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, errors.New("due_date inválida, se espera YYYY-MM-DD")
	}
	utc := t.UTC()
	return &utc, nil
}
