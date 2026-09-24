package dashboard

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// ActivityLog es un buffer circular de eventos mostrado en la vista de logs.
// También puede actuar como slog.Handler para capturar los logs del dashboard.
type ActivityLog struct {
	mu    sync.Mutex
	lines []string
	max   int
}

// NewActivityLog crea un buffer con capacidad máxima.
func NewActivityLog(max int) *ActivityLog {
	if max <= 0 {
		max = 500
	}
	return &ActivityLog{max: max}
}

// Add registra un evento con marca de tiempo.
func (l *ActivityLog) Add(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, fmt.Sprintf("%s  %s", time.Now().Format("15:04:05"), msg))
	if len(l.lines) > l.max {
		l.lines = l.lines[len(l.lines)-l.max:]
	}
}

// Lines devuelve una copia de los eventos.
func (l *ActivityLog) Lines() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]string, len(l.lines))
	copy(out, l.lines)
	return out
}

// Handler devuelve un slog.Handler que escribe en este buffer.
func (l *ActivityLog) Handler() slog.Handler { return &activityHandler{log: l} }

type activityHandler struct {
	log   *ActivityLog
	attrs []slog.Attr
}

func (h *activityHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *activityHandler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	b.WriteString(r.Message)
	for _, a := range h.attrs {
		b.WriteString(" " + a.Key + "=" + a.Value.String())
	}
	r.Attrs(func(a slog.Attr) bool {
		b.WriteString(" " + a.Key + "=" + a.Value.String())
		return true
	})
	h.log.Add("[%s] %s", r.Level.String(), b.String())
	return nil
}

func (h *activityHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &activityHandler{log: h.log, attrs: append(append([]slog.Attr{}, h.attrs...), attrs...)}
}

func (h *activityHandler) WithGroup(_ string) slog.Handler { return h }
