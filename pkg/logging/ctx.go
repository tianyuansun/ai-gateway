package logging

import (
	"context"
	"log/slog"
)

type ctxKey struct{}

// WithLogger creates a new logger backed by a buffering handler, injects it into ctx,
// and returns the enriched context along with the buffer for later flush/discard.
func WithLogger(ctx context.Context, requestID string, level slog.Level) (context.Context, *Buffer) {
	buf := NewBuffer(256)
	h := newHandler(buf, requestID, level)
	logger := slog.New(h)
	return context.WithValue(ctx, ctxKey{}, logger), buf
}

// LoggerFrom extracts the logger from ctx. Returns nil if none is present.
func LoggerFrom(ctx context.Context) *slog.Logger {
	if v := ctx.Value(ctxKey{}); v != nil {
		return v.(*slog.Logger)
	}
	return nil
}
