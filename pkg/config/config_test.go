package config

import (
	"testing"
)

func TestServerConfigLogDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()

	if cfg.Server.LogLevel != "info" {
		t.Errorf("default log_level should be info, got %q", cfg.Server.LogLevel)
	}
	if cfg.Server.LogLatencyThresholdMs != 5000 {
		t.Errorf("default log_latency_threshold_ms should be 5000, got %d", cfg.Server.LogLatencyThresholdMs)
	}
}
