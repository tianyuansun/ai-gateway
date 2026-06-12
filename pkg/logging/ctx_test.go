package logging

import (
	"context"
	"log/slog"
	"testing"
)

func TestWithLoggerAndLoggerFrom(t *testing.T) {
	ctx := context.Background()
	newCtx, buf := WithLogger(ctx, "req-1", slog.LevelInfo)

	// ctx should not be the same
	if newCtx == ctx {
		t.Error("expected new context, got original")
	}

	// LoggerFrom should return non-nil
	logger := LoggerFrom(newCtx)
	if logger == nil {
		t.Fatal("expected logger from ctx, got nil")
	}

	// Logging should work and populate buffer
	logger.Info("test message", "key", "val")
	if buf.Len() != 1 {
		t.Errorf("expected 1 record in buffer, got %d", buf.Len())
	}

	// LoggerFrom on bare context returns nil
	if LoggerFrom(ctx) != nil {
		t.Error("expected nil logger from bare context")
	}
}
