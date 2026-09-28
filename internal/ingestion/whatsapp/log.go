package whatsapp

import (
	"context"
	"log/slog"

	waLog "go.mau.fi/whatsmeow/util/log"
)

// slogLogger adapta el logger de whatsmeow al slog del proyecto, evitando
// escrituras directas a stdout y manteniendo el log estructurado.
type slogLogger struct {
	logger *slog.Logger
	module string
}

func newSlogLogger(logger *slog.Logger, module string) waLog.Logger {
	if logger == nil {
		logger = slog.Default()
	}
	return &slogLogger{logger: logger, module: module}
}

func (l *slogLogger) log(level slog.Level, msg string, args ...any) {
	l.logger.Log(context.Background(), level, msg, append([]any{"module", l.module}, args...)...)
}

func (l *slogLogger) Errorf(msg string, args ...any) { l.log(slog.LevelError, msg, args...) }
func (l *slogLogger) Warnf(msg string, args ...any)  { l.log(slog.LevelWarn, msg, args...) }
func (l *slogLogger) Infof(msg string, args ...any)  { l.log(slog.LevelInfo, msg, args...) }
func (l *slogLogger) Debugf(msg string, args ...any) { l.log(slog.LevelDebug, msg, args...) }

func (l *slogLogger) Sub(module string) waLog.Logger {
	return &slogLogger{logger: l.logger, module: l.module + "." + module}
}
