package logging

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"
)

func TestTriggerXDebugFlushes(t *testing.T) {
	_, buf := WithLogger(context.Background(), "req-1", slog.LevelInfo)
	logger := slog.New(newHandler(buf, "req-1", slog.LevelInfo))
	logger.Info("some log")

	cfg := FlushConfig{XDebug: true}
	if !cfg.ShouldFlush(10*time.Millisecond, 200, nil) {
		t.Error("X-Debug should trigger flush")
	}
}

func TestTriggerErrorStatusFlushes(t *testing.T) {
	cfg := FlushConfig{}
	if !cfg.ShouldFlush(0, 500, nil) {
		t.Error("status 500 should trigger flush")
	}
	if !cfg.ShouldFlush(0, 502, nil) {
		t.Error("status 502 should trigger flush")
	}
}

func TestTriggerUpstreamErrFlushes(t *testing.T) {
	cfg := FlushConfig{}
	if !cfg.ShouldFlush(0, 0, errors.New("connection refused")) {
		t.Error("upstream error should trigger flush")
	}
}

func TestTriggerLatencyExceedsThresholdFlushes(t *testing.T) {
	cfg := FlushConfig{Threshold: 100 * time.Millisecond}
	// 150ms > 100ms threshold → flush
	if !cfg.ShouldFlush(150*time.Millisecond, 200, nil) {
		t.Error("latency above threshold should trigger flush")
	}
}

func TestTriggerLatencyBelowThresholdDiscards(t *testing.T) {
	cfg := FlushConfig{Threshold: 100 * time.Millisecond}
	// 50ms < 100ms threshold → no flush
	if cfg.ShouldFlush(50*time.Millisecond, 200, nil) {
		t.Error("latency below threshold should not trigger flush")
	}
}

func TestTriggerCancelErrorFlushes(t *testing.T) {
	cfg := FlushConfig{}
	// context.Canceled → should flush
	if !cfg.ShouldFlush(0, 0, context.Canceled) {
		t.Error("context.Canceled should trigger flush")
	}
	// context.DeadlineExceeded → should flush
	if !cfg.ShouldFlush(0, 0, context.DeadlineExceeded) {
		t.Error("context.DeadlineExceeded should trigger flush")
	}
}

func TestTriggerNormalRequestDiscards(t *testing.T) {
	_, buf := WithLogger(context.Background(), "req-2", slog.LevelInfo)
	logger := slog.New(newHandler(buf, "req-2", slog.LevelInfo))
	logger.Info("ok")

	cfg := FlushConfig{}
	if cfg.ShouldFlush(50*time.Millisecond, 200, nil) {
		t.Error("normal request should not trigger flush")
	}
}
