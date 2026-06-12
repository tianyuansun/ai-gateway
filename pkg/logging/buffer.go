package logging

import (
	"encoding/json"
	"io"
	"log/slog"
	"time"
)

// JSON key constants matching slog's JSONHandler output.
const (
	LogKeyLevel = "level"
	LogKeyMsg   = "msg"
	LogKeyTime  = "time"
)

// Buffer is a fixed-capacity ring buffer of slog.Record values.
type Buffer struct {
	records  []slog.Record
	capacity int
	pos      int // next write position
	full     bool
}

// NewBuffer creates a Buffer with the given capacity.
func NewBuffer(capacity int) *Buffer {
	return &Buffer{
		records:  make([]slog.Record, capacity),
		capacity: capacity,
	}
}

// Add appends a record. If the buffer is full, the oldest record is overwritten.
func (b *Buffer) Add(r slog.Record) {
	b.records[b.pos] = r
	b.pos = (b.pos + 1) % b.capacity
	if b.pos == 0 {
		b.full = true
	}
}

// Len returns the number of records currently in the buffer.
func (b *Buffer) Len() int {
	if b.full {
		return b.capacity
	}
	return b.pos
}

// Records returns the buffered records in insertion order (oldest first).
func (b *Buffer) Records() []slog.Record {
	n := b.Len()
	out := make([]slog.Record, n)
	if b.full {
		for i := range n {
			out[i] = b.records[(b.pos+i)%b.capacity]
		}
	} else {
		copy(out, b.records[:b.pos])
	}
	return out
}

// Flush writes all buffered records as JSON Lines to w, then clears the buffer.
func (b *Buffer) Flush(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	for _, r := range b.Records() {
		m := map[string]any{
			LogKeyTime: r.Time.Format(time.RFC3339Nano),
			LogKeyMsg:  r.Message,
		}
		// Determine level string.
		levelStr := "INFO"
		switch {
		case r.Level >= slog.LevelError:
			levelStr = "ERROR"
		case r.Level >= slog.LevelWarn:
			levelStr = "WARN"
		case r.Level >= slog.LevelInfo:
			levelStr = "INFO"
		default:
			levelStr = "DEBUG"
		}
		m[LogKeyLevel] = levelStr
		// Copy attrs.
		r.Attrs(func(a slog.Attr) bool {
			m[a.Key] = a.Value.Any()
			return true
		})
		if err := enc.Encode(m); err != nil {
			return err
		}
	}
	b.Discard()
	return nil
}

// Discard clears the buffer.
func (b *Buffer) Discard() {
	b.pos = 0
	b.full = false
	// Clear refs to allow GC.
	for i := range b.records {
		b.records[i] = slog.Record{}
	}
}
