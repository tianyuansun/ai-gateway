package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestBufferStoresRecordsUpToCapacity(t *testing.T) {
	buf := NewBuffer(3)

	if buf.Len() != 0 {
		t.Errorf("new buffer should be empty, got %d records", buf.Len())
	}

	buf.Add(slogRecord("msg1"))
	buf.Add(slogRecord("msg2"))
	buf.Add(slogRecord("msg3"))

	if buf.Len() != 3 {
		t.Errorf("expected 3 records, got %d", buf.Len())
	}
}

func TestBufferRingOverwrite(t *testing.T) {
	buf := NewBuffer(2)

	buf.Add(slogRecord("a"))
	buf.Add(slogRecord("b"))
	buf.Add(slogRecord("c")) // overwrites "a"

	if buf.Len() != 2 {
		t.Errorf("expected 2 records, got %d", buf.Len())
	}

	records := buf.Records()
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0].Message != "b" {
		t.Errorf("expected first record to be 'b', got %q", records[0].Message)
	}
	if records[1].Message != "c" {
		t.Errorf("expected second record to be 'c', got %q", records[1].Message)
	}
}

func TestHandlerLevelFiltering(t *testing.T) {
	buf := NewBuffer(10)
	h := newHandler(buf, "req-1", slog.LevelInfo)
	logger := slog.New(h)

	logger.Debug("debug msg")
	if buf.Len() != 0 {
		t.Errorf("debug record should be dropped when handler level is info, got %d records", buf.Len())
	}

	logger.Info("info msg")
	if buf.Len() != 1 {
		t.Errorf("info record should be kept, got %d records", buf.Len())
	}
}

// Helper: create a minimal slog.Record for testing.
func slogRecord(msg string) slog.Record {
	return slogRecordAt(slog.LevelInfo, msg)
}

func TestFlushProducesJSONLines(t *testing.T) {
	buf := NewBuffer(3)
	h := newHandler(buf, "gw-abc123", slog.LevelDebug)
	logger := slog.New(h)

	logger.Info("first", "key", "val1")
	logger.Warn("second", "count", 42)

	var out bytes.Buffer
	err := buf.Flush(&out)
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 JSON lines, got %d: %s", len(lines), out.String())
	}

	for i, line := range lines {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Errorf("line %d: invalid JSON: %v", i, err)
		}
		if m["request_id"] != "gw-abc123" {
			t.Errorf("line %d: expected request_id gw-abc123, got %v", i, m["request_id"])
		}
		if m[logKeyLevel] == nil {
			t.Errorf("line %d: missing level", i)
		}
		if m[logKeyMsg] == nil {
			t.Errorf("line %d: missing msg", i)
		}
		if m[logKeyTime] == nil {
			t.Errorf("line %d: missing time", i)
		}
	}
}

// JSON key constants for slog.
const (
	logKeyLevel = "level"
	logKeyMsg   = "msg"
	logKeyTime  = "time"
)

func TestWithLoggerIntegratesWithSlog(t *testing.T) {
	// End-to-end: create logger, write at different levels, flush, verify output.
	ctx := context.Background()
	newCtx, buf := WithLogger(ctx, "req-int", slog.LevelDebug)

	logger := LoggerFrom(newCtx)
	logger.Debug("debug detail", "step", 1)
	logger.Info("info message", "user", "alice")
	logger.Warn("warning", "retry", 2)
	logger.Error("something failed", "code", 500)

	if buf.Len() != 4 {
		t.Errorf("expected 4 records at debug level, got %d", buf.Len())
	}

	var out bytes.Buffer
	buf.Flush(&out)

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 JSON lines, got %d", len(lines))
	}

	for _, line := range lines {
		var m map[string]any
		json.Unmarshal([]byte(line), &m)
		if m["request_id"] != "req-int" {
			t.Errorf("expected request_id req-int, got %v", m["request_id"])
		}
	}

	// After flush, buffer should be empty.
	if buf.Len() != 0 {
		t.Errorf("buffer should be empty after flush, got %d", buf.Len())
	}
}

func slogRecordAt(level slog.Level, msg string) slog.Record {
	var r slog.Record
	r.Message = msg
	r.Time = time.Now()
	r.Level = level
	return r
}
