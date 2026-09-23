package anonymizer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Config ajusta el servidor HTTP del anonimizador.
type Config struct {
	Token          string        // token Bearer interno; vacío desactiva la comprobación
	MaxBytes       int64         // límite de tamaño por petición
	ProcessingTTL  time.Duration // TTL de la MappingTable
	ProcessingTime time.Duration // límite de procesamiento
}

// Server expone /api/sanitize y /api/revert.
type Server struct {
	engine *Engine
	store  MappingStore
	cfg    Config
	logger *slog.Logger
}

// NewServer crea el servidor HTTP.
func NewServer(engine *Engine, store MappingStore, cfg Config, logger *slog.Logger) *Server {
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = 32 << 10 // 32 KiB
	}
	if cfg.ProcessingTTL <= 0 {
		cfg.ProcessingTTL = 5 * time.Minute
	}
	if cfg.ProcessingTime <= 0 {
		cfg.ProcessingTime = 10 * time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{engine: engine, store: store, cfg: cfg, logger: logger}
}

// Handler devuelve el router con autenticación interna.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/sanitize", s.auth(s.handleSanitize))
	mux.HandleFunc("POST /api/revert", s.auth(s.handleRevert))
	return mux
}

var tokenRe = regexp.MustCompile(`\{\{[A-Za-z0-9_]+\}\}`)

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.Token != "" {
			auth := r.Header.Get("Authorization")
			if !strings.EqualFold(auth, "Bearer "+s.cfg.Token) {
				s.writeError(w, http.StatusUnauthorized, "unauthorized", "Credenciales ausentes o inválidas")
				return
			}
		}
		next(w, r)
	}
}

func (s *Server) handleSanitize(w http.ResponseWriter, r *http.Request) {
	if !requireJSON(w, r) {
		return
	}
	body, ok := s.readBody(w, r)
	if !ok {
		return
	}

	var req SanitizeRequest
	if err := json.Unmarshal(body, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "La petición no cumple el contrato")
		return
	}
	if strings.TrimSpace(req.Prompt) == "" {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "El campo prompt es obligatorio")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.ProcessingTime)
	defer cancel()

	clean, entities, mapping := s.engine.Sanitize(req.Prompt)

	requestID := uuid.NewString()
	if err := s.store.Save(ctx, requestID, mapping, s.cfg.ProcessingTTL); err != nil {
		s.logger.Error("guardar mapping", "error", err)
		s.writeError(w, http.StatusServiceUnavailable, "unavailable", "No disponible temporalmente")
		return
	}

	s.writeJSON(w, http.StatusOK, SanitizeResponse{
		RequestID:   requestID,
		CleanPrompt: clean,
		Entities:    entities,
	})
}

func (s *Server) handleRevert(w http.ResponseWriter, r *http.Request) {
	if !requireJSON(w, r) {
		return
	}
	body, ok := s.readBody(w, r)
	if !ok {
		return
	}

	var req RevertRequest
	if err := json.Unmarshal(body, &req); err != nil || req.RequestID == "" {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "La petición no cumple el contrato")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.ProcessingTime)
	defer cancel()

	mapping, err := s.store.Load(ctx, req.RequestID)
	if errors.Is(err, ErrMappingNotFound) {
		s.writeError(w, http.StatusNotFound, "not_found", "request_id inexistente o expirado")
		return
	}
	if err != nil {
		s.logger.Error("cargar mapping", "error", err)
		s.writeError(w, http.StatusServiceUnavailable, "unavailable", "No disponible temporalmente")
		return
	}

	reverted, err := applyMapping(req.ProcessedText, mapping)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Tokens desconocidos o malformados")
		return
	}

	if err := s.store.Delete(ctx, req.RequestID); err != nil {
		s.logger.Error("eliminar mapping", "error", err)
	}

	s.writeJSON(w, http.StatusOK, RevertResponse{
		RequestID:    req.RequestID,
		RevertedText: reverted,
	})
}

// applyMapping sustituye tokens conocidos; rechaza tokens desconocidos.
func applyMapping(text string, mapping MappingTable) (string, error) {
	seen := tokenRe.FindAllString(text, -1)
	for _, tok := range seen {
		if _, ok := mapping[tok]; !ok {
			return "", errors.New("token desconocido")
		}
	}
	out := text
	for tok, val := range mapping {
		out = strings.ReplaceAll(out, tok, val)
	}
	return out, nil
}

func requireJSON(w http.ResponseWriter, r *http.Request) bool {
	ct := r.Header.Get("Content-Type")
	if !strings.HasPrefix(strings.ToLower(ct), "application/json") {
		writeJSONError(w, http.StatusBadRequest, "invalid_request", "Content-Type debe ser application/json")
		return false
	}
	return true
}

func (s *Server) readBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "Prompt demasiado grande")
		return nil, false
	}
	return body, true
}

func (s *Server) writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSONError(w, status, code, message)
}

func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Error: ErrorBody{Code: code, Message: message}})
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
