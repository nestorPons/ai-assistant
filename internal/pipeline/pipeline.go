// Package pipeline orquesta el flujo end-to-end: autorización, registro,
// ofuscación, contexto, clasificación (Jev) y extracción (LLM).
package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/nestorPons/ai-assistant/internal/anonymizer"
	"github.com/nestorPons/ai-assistant/internal/classifier"
	ctxbuilder "github.com/nestorPons/ai-assistant/internal/context"
	"github.com/nestorPons/ai-assistant/internal/domain"
	"github.com/nestorPons/ai-assistant/internal/extractor"
	"github.com/nestorPons/ai-assistant/internal/normalizer"
	"github.com/nestorPons/ai-assistant/internal/persistence"
)

// Config ajusta umbrales y pre-filtro del pipeline.
type Config struct {
	MinWords              int
	ExtractConfidence     float64
	ReviewBelowConfidence float64
}

// Pipeline ejecuta el procesamiento de mensajes autorizados.
type Pipeline struct {
	store      persistence.Store
	normalizer *normalizer.Normalizer
	anonymizer *anonymizer.Client
	context    *ctxbuilder.Builder
	classifier classifier.Classifier
	extractor  extractor.Extractor
	logger     *slog.Logger
	cfg        Config
}

// New construye el pipeline.
func New(store persistence.Store, anon *anonymizer.Client, cls classifier.Classifier, ext extractor.Extractor, cfg Config, logger *slog.Logger) *Pipeline {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.MinWords <= 0 {
		cfg.MinWords = 3
	}
	if cfg.ExtractConfidence <= 0 {
		cfg.ExtractConfidence = 0.6
	}
	if cfg.ReviewBelowConfidence <= 0 {
		cfg.ReviewBelowConfidence = 0.5
	}
	return &Pipeline{
		store:      store,
		normalizer: normalizer.New(),
		anonymizer: anon,
		context:    ctxbuilder.New(),
		classifier: cls,
		extractor:  ext,
		logger:     logger,
		cfg:        cfg,
	}
}

// Process gestiona un mensaje entrante aplicando el orden obligatorio del flujo.
// Devuelve error solo para fallos reintentables; la exclusión y el descarte
// devuelven nil.
func (p *Pipeline) Process(ctx context.Context, msg domain.IncomingMessage) error {
	if err := p.normalizer.Normalize(ctx, &msg); err != nil {
		p.logger.Warn("normalizacion rechazada", "source", msg.Source, "error", err)
		return nil
	}

	// 1. Filtro de usuarios autorizados (lista blanca).
	client, err := p.store.FindBySourceAndIdentifier(ctx, msg.Source, msg.ClientIdentifier)
	if errors.Is(err, persistence.ErrNotFound) {
		p.logger.Debug("usuario excluido (no registrado)", "source", msg.Source)
		return nil
	}
	if err != nil {
		return err
	}
	if !client.Authorized() {
		p.logger.Debug("usuario excluido (no autorizado)", "source", msg.Source)
		return nil
	}

	// 2. Idempotencia.
	exists, err := p.store.ExistsBySourceAndExternalID(ctx, msg.Source, msg.ID)
	if err != nil {
		return err
	}
	if exists {
		p.logger.Debug("mensaje duplicado, ignorado", "source", msg.Source)
		return nil
	}

	// 3. Registro protegido del original autorizado.
	raw := &domain.RawMessage{
		ClientID:   client.ID,
		Source:     msg.Source,
		ExternalID: msg.ID,
		Content:    msg.RawContent,
	}
	if err := p.store.CreateRawMessage(ctx, raw); err != nil {
		if errors.Is(err, persistence.ErrDuplicate) {
			return nil
		}
		return err
	}

	// 4. Pre-filtro ligero.
	if normalizer.WordCount(msg.RawContent) < p.cfg.MinWords {
		p.logger.Debug("mensaje descartado por pre-filtro", "message_id", raw.ID)
		return p.store.MarkProcessed(ctx, raw.ID)
	}

	// 5. Ofuscación de datos personales.
	clean, err := p.anonymizer.Sanitize(ctx, anonymizer.SanitizeRequest{Prompt: msg.RawContent})
	if err != nil {
		return fmt.Errorf("ofuscación: %w", err)
	}

	// 6. Construcción de contexto controlado.
	ctxJSON, err := p.context.Build(msg, clean.CleanPrompt)
	if err != nil {
		return err
	}

	// 7. Clasificación con Jev.
	clsResult, err := p.classifier.Classify(ctx, classifier.ClassificationInput{
		CleanPrompt: ctxJSON,
		Source:      msg.Source,
	})
	if err != nil {
		// Aviso técnico; no se ejecutan acciones automáticas.
		p.logger.Error("clasificacion fallida", "message_id", raw.ID, "error", err)
		return err
	}

	if clsResult.Blocked {
		p.logger.Warn("procesamiento bloqueado por Jev", "message_id", raw.ID, "reason", clsResult.Reason)
		return p.store.MarkProcessed(ctx, raw.ID)
	}
	if clsResult.NeedsReview {
		p.logger.Warn("mensaje pendiente de revisión", "message_id", raw.ID, "reason", clsResult.Reason)
		return p.store.MarkProcessed(ctx, raw.ID)
	}

	// 8. Enrutamiento interno.
	switch clsResult.Decision {
	case classifier.DecisionDiscard:
		p.logger.Debug("mensaje descartado", "message_id", raw.ID, "confidence", clsResult.Confidence)
		return p.store.MarkProcessed(ctx, raw.ID)

	case classifier.DecisionExtract:
		return p.extractAndStore(ctx, raw, msg.Source, clean.CleanPrompt)

	case classifier.DecisionDBAction:
		return p.applyDBAction(ctx, raw, clsResult)

	default:
		p.logger.Warn("decisión desconocida", "message_id", raw.ID, "decision", clsResult.Decision)
		return p.store.MarkProcessed(ctx, raw.ID)
	}
}

// extractAndStore ejecuta el extractor y persiste la tarea resultante.
func (p *Pipeline) extractAndStore(ctx context.Context, raw *domain.RawMessage, source domain.Source, cleanPrompt string) error {
	res, err := p.extractor.Extract(ctx, extractor.ExtractionInput{CleanPrompt: cleanPrompt, Source: source})
	if err != nil {
		p.logger.Error("extracción fallida", "message_id", raw.ID, "error", err)
		return err
	}

	if !res.IsTask {
		p.logger.Debug("mensaje sin tarea", "message_id", raw.ID)
		return p.store.MarkProcessed(ctx, raw.ID)
	}

	status := domain.TaskStatusPending
	if res.Confidence < p.cfg.ReviewBelowConfidence {
		status = domain.TaskStatusNeedsReview
	}

	task := &domain.Task{
		ClientID:       raw.ClientID,
		MessageID:      raw.ID,
		Title:          res.Title,
		Description:    res.Description,
		Subject:        res.Subject,
		Priority:       res.Priority,
		EstimatedHours: res.EstimatedHours,
		DueDate:        res.DueDate,
		Specifications: res.Specifications,
		Status:         status,
		AIConfidence:   res.Confidence,
	}

	if err := p.store.CreateTask(ctx, task); err != nil {
		return err
	}

	p.logger.Info("tarea creada", "task_id", task.ID, "message_id", raw.ID, "status", status, "confidence", res.Confidence)
	return p.store.MarkProcessed(ctx, raw.ID)
}

// applyDBAction valida y ejecuta acciones tipadas (create/update) mediante
// repositorios. Nunca ejecuta SQL generado por el modelo.
func (p *Pipeline) applyDBAction(ctx context.Context, raw *domain.RawMessage, cls classifier.ClassificationResult) error {
	a := cls.Action
	if a == nil {
		p.logger.Warn("db_action sin acción", "message_id", raw.ID)
		return p.store.MarkProcessed(ctx, raw.ID)
	}

	switch a.Entity {
	case "client":
		return p.applyClientAction(ctx, raw, a)
	case "task":
		return p.applyTaskAction(ctx, raw, a)
	default:
		p.logger.Warn("entidad de acción no soportada", "message_id", raw.ID, "entity", a.Entity)
		return p.store.MarkProcessed(ctx, raw.ID)
	}
}

func (p *Pipeline) applyClientAction(ctx context.Context, raw *domain.RawMessage, a *classifier.Action) error {
	switch a.Operation {
	case "create":
		source, _ := a.Fields["source"].(string)
		identifier, _ := a.Fields["identifier"].(string)
		if source == "" || identifier == "" {
			p.logger.Warn("acción create client incompleta", "message_id", raw.ID)
			return p.store.MarkProcessed(ctx, raw.ID)
		}
		client := &domain.Client{
			Source:     domain.Source(source),
			Identifier: identifier,
			Name:       strField(a.Fields, "name"),
			Tracked:    boolField(a.Fields, "tracked", true),
			Active:     boolField(a.Fields, "active", true),
		}
		if err := p.store.CreateClient(ctx, client); err != nil {
			return err
		}
		p.logger.Info("cliente creado", "client_id", client.ID, "message_id", raw.ID)
	case "update":
		id := a.TargetID
		if id == "" {
			id = strField(a.Fields, "id")
		}
		if id == "" {
			p.logger.Warn("acción update client sin id", "message_id", raw.ID)
			return p.store.MarkProcessed(ctx, raw.ID)
		}
		// Sólo se permite alternar tracked/active o renombrar; operación segura.
		existing, err := p.store.FindBySourceAndIdentifier(ctx, domain.Source(strField(a.Fields, "source")), strField(a.Fields, "identifier"))
		if err != nil {
			p.logger.Warn("update client: cliente no encontrado", "message_id", raw.ID, "error", err)
			return p.store.MarkProcessed(ctx, raw.ID)
		}
		if v, ok := a.Fields["tracked"].(bool); ok {
			existing.Tracked = v
		}
		if v, ok := a.Fields["active"].(bool); ok {
			existing.Active = v
		}
		if v := strField(a.Fields, "name"); v != "" {
			existing.Name = v
		}
		if err := p.store.UpdateClient(ctx, existing); err != nil {
			return err
		}
		p.logger.Info("cliente actualizado", "client_id", existing.ID, "message_id", raw.ID)
	default:
		p.logger.Warn("operación client no soportada", "message_id", raw.ID, "operation", a.Operation)
	}
	return p.store.MarkProcessed(ctx, raw.ID)
}

func (p *Pipeline) applyTaskAction(ctx context.Context, raw *domain.RawMessage, a *classifier.Action) error {
	if a.Operation != "update" {
		p.logger.Warn("operación task no soportada", "message_id", raw.ID, "operation", a.Operation)
		return p.store.MarkProcessed(ctx, raw.ID)
	}
	id := a.TargetID
	if id == "" {
		id = strField(a.Fields, "id")
	}
	if id == "" {
		p.logger.Warn("acción update task sin id", "message_id", raw.ID)
		return p.store.MarkProcessed(ctx, raw.ID)
	}
	if s, ok := a.Fields["status"].(string); ok && domain.TaskStatus(s).Valid() {
		if err := p.store.UpdateTaskStatus(ctx, id, domain.TaskStatus(s)); err != nil {
			return err
		}
		p.logger.Info("tarea actualizada", "task_id", id, "message_id", raw.ID)
	}
	return p.store.MarkProcessed(ctx, raw.ID)
}

func strField(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func boolField(m map[string]any, key string, def bool) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return def
}
