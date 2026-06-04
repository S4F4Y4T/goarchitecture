// Package logger provides the application's structured logger built on
// log/slog. It exposes a process-wide default logger plus context helpers so a
// request-scoped logger (carrying the request ID) can flow through the
// middleware chain and into handlers.
package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

// defaultLogger is the process-wide logger used when no request-scoped logger
// is present in the context. Init replaces it; until then it is a no-frills
// JSON logger so logging is safe before Init runs.
var defaultLogger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

type ctxKey struct{}

// Init configures the process-wide structured logger to emit JSON at the given
// level to w (typically os.Stdout) and installs it as both the package default
// and slog's package-level default, so bare slog.Info/Error calls are
// structured too. It returns the logger for convenience.
func Init(w io.Writer, level slog.Level) *slog.Logger {
	l := slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))
	defaultLogger = l
	slog.SetDefault(l)
	return l
}

// L returns the process-wide default logger.
func L() *slog.Logger {
	return defaultLogger
}

// WithContext returns a copy of ctx carrying l, so downstream code can retrieve
// a request-scoped logger via FromContext.
func WithContext(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext returns the request-scoped logger stored in ctx, or the default
// logger when none is present (or ctx is nil).
func FromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return defaultLogger
	}
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return defaultLogger
}

// ParseLevel maps a level name ("debug", "info", "warn", "error") to an
// slog.Level, falling back to Info for empty or unrecognized values.
func ParseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
