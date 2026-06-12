package logging

import (
	"log/slog"
	"testing"
)

func TestGlobalLevelDefaultIsInfo(t *testing.T) {
	if GlobalLevel() != slog.LevelInfo {
		t.Errorf("default global level should be info, got %v", GlobalLevel())
	}
}

func TestSetGlobalLevelUpdatesLevel(t *testing.T) {
	orig := GlobalLevel()
	defer SetGlobalLevel(orig)

	SetGlobalLevel(slog.LevelDebug)
	if GlobalLevel() != slog.LevelDebug {
		t.Errorf("expected debug after SetGlobalLevel, got %v", GlobalLevel())
	}

	SetGlobalLevel(slog.LevelError)
	if GlobalLevel() != slog.LevelError {
		t.Errorf("expected error after SetGlobalLevel, got %v", GlobalLevel())
	}
}
