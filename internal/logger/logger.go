package logger

import (
	"context"
	"log/slog"
)

type contextKey struct{}

// WithContext stores the logger in the context.
func WithContext(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, l)
}

// FromContext retrieves the logger from context, falling back to the default logger.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(contextKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}

// WithRequestID returns a new logger enriched with the request_id field.
func WithRequestID(l *slog.Logger, requestID string) *slog.Logger {
	return l.With("request_id", requestID)
}

// WithTraceID returns a new logger enriched with the trace_id field.
func WithTraceID(l *slog.Logger, traceID string) *slog.Logger {
	return l.With("trace_id", traceID)
}
