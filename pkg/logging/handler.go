package logging

import (
	"context"
	"log/slog"
)

// handler implements slog.Handler with buffered storage.
type handler struct {
	buf       *Buffer
	requestID string
	level     slog.Level
}

func newHandler(buf *Buffer, requestID string, level slog.Level) *handler {
	return &handler{
		buf:       buf,
		requestID: requestID,
		level:     level,
	}
}

// Enabled reports whether the handler is enabled for the given level.
func (h *handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

// Handle stores the record in the buffer.
func (h *handler) Handle(_ context.Context, r slog.Record) error {
	r.AddAttrs(slog.String("request_id", h.requestID))
	h.buf.Add(r)
	return nil
}

// WithAttrs returns a new handler with the given attrs (not used for per-request logging).
func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

// WithGroup returns a new handler with the given group (not used).
func (h *handler) WithGroup(name string) slog.Handler {
	return h
}
