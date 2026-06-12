package logging

import (
	"log/slog"
	"sync/atomic"
)

var globalLevel atomic.Int32

func init() {
	globalLevel.Store(int32(slog.LevelInfo))
}

// GlobalLevel returns the current global log level.
func GlobalLevel() slog.Level {
	return slog.Level(globalLevel.Load())
}

// SetGlobalLevel sets the global log level (thread-safe).
func SetGlobalLevel(level slog.Level) {
	globalLevel.Store(int32(level))
}
