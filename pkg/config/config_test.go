package config

import (
	"os"
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

func TestLoadFullYAML(t *testing.T) {
	yamlData := `
server:
  listen: "0.0.0.0:8080"
  log_level: debug
  log_latency_threshold_ms: 3000
  session:
    ttl_seconds: 7200
    backend: redis
    key_source: manual

providers:
  openai:
    endpoints:
      chat: "https://api.openai.com"
      anthropic: ""
    api_key_env: OPENAI_API_KEY
  anthropic:
    endpoints:
      chat: ""
      anthropic: "https://api.anthropic.com"
    api_key_env: ANTHROPIC_API_KEY

models:
  gpt-4:
    aliases: ["gpt4"]
    display_name: "GPT-4"
    capabilities:
      context_window: 128000
      max_output_tokens: 4096
      supports_tools: true
      supports_parallel_tool_calls: true
      supports_vision: true
      supports_reasoning: false
      input_modalities: ["text", "image"]
    routing:
      strategy: weighted
      affinity: sticky
    providers:
      - provider: openai
        priority: 1
  claude-3:
    aliases: ["claude3"]
    display_name: "Claude 3"
    capabilities:
      context_window: 200000
      max_output_tokens: 4096
      supports_tools: true
      supports_parallel_tool_calls: true
      supports_vision: true
      supports_reasoning: true
      input_modalities: ["text", "image"]
    routing:
      strategy: priority
      affinity: sticky
    providers:
      - provider: anthropic
        priority: 1

routing:
  strategy: priority
  affinity: sticky
`
	filePath := "/tmp/test_full_config.yaml"
	if err := os.WriteFile(filePath, []byte(yamlData), 0644); err != nil {
		t.Fatalf("failed to write temp config file: %v", err)
	}
	defer os.Remove(filePath)

	cfg, err := Load(filePath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Server checks
	if cfg.Server.Listen != "0.0.0.0:8080" {
		t.Errorf("Server.Listen: expected '0.0.0.0:8080', got %q", cfg.Server.Listen)
	}
	if cfg.Server.LogLevel != "debug" {
		t.Errorf("Server.LogLevel: expected 'debug', got %q", cfg.Server.LogLevel)
	}
	if cfg.Server.LogLatencyThresholdMs != 3000 {
		t.Errorf("Server.LogLatencyThresholdMs: expected 3000, got %d", cfg.Server.LogLatencyThresholdMs)
	}
	if cfg.Server.Session.TTLSeconds != 7200 {
		t.Errorf("Session.TTLSeconds: expected 7200, got %d", cfg.Server.Session.TTLSeconds)
	}
	if cfg.Server.Session.Backend != "redis" {
		t.Errorf("Session.Backend: expected 'redis', got %q", cfg.Server.Session.Backend)
	}
	if cfg.Server.Session.KeySource != "manual" {
		t.Errorf("Session.KeySource: expected 'manual', got %q", cfg.Server.Session.KeySource)
	}

	// Providers checks
	if len(cfg.Providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(cfg.Providers))
	}
	p, ok := cfg.Providers["openai"]
	if !ok {
		t.Fatal("expected 'openai' provider")
	}
	if p.Endpoints.Chat != "https://api.openai.com" {
		t.Errorf("openai Chat endpoint: got %q", p.Endpoints.Chat)
	}
	if p.APIKeyEnv != "OPENAI_API_KEY" {
		t.Errorf("openai APIKeyEnv: got %q", p.APIKeyEnv)
	}
	p, ok = cfg.Providers["anthropic"]
	if !ok {
		t.Fatal("expected 'anthropic' provider")
	}
	if p.Endpoints.Anthropic != "https://api.anthropic.com" {
		t.Errorf("anthropic endpoint: got %q", p.Endpoints.Anthropic)
	}
	if p.APIKeyEnv != "ANTHROPIC_API_KEY" {
		t.Errorf("anthropic APIKeyEnv: got %q", p.APIKeyEnv)
	}

	// Models checks
	if len(cfg.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(cfg.Models))
	}
	m, ok := cfg.Models["gpt-4"]
	if !ok {
		t.Fatal("expected 'gpt-4' model")
	}
	if m.DisplayName != "GPT-4" {
		t.Errorf("gpt-4 DisplayName: got %q", m.DisplayName)
	}
	if len(m.Aliases) != 1 || m.Aliases[0] != "gpt4" {
		t.Errorf("gpt-4 Aliases: got %v", m.Aliases)
	}
	if m.Capabilities.ContextWindow != 128000 {
		t.Errorf("gpt-4 ContextWindow: got %d", m.Capabilities.ContextWindow)
	}
	if m.Capabilities.SupportsTools != true {
		t.Error("gpt-4 SupportsTools should be true")
	}
	if m.Capabilities.SupportsReasoning != false {
		t.Error("gpt-4 SupportsReasoning should be false")
	}
	if m.Routing == nil || m.Routing.Strategy != "weighted" {
		t.Errorf("gpt-4 Routing.Strategy: got %v", m.Routing)
	}
	if len(m.Providers) != 1 || m.Providers[0].Provider != "openai" {
		t.Errorf("gpt-4 Providers: got %v", m.Providers)
	}

	m, ok = cfg.Models["claude-3"]
	if !ok {
		t.Fatal("expected 'claude-3' model")
	}
	if m.Capabilities.SupportsReasoning != true {
		t.Error("claude-3 SupportsReasoning should be true")
	}
	if len(m.Capabilities.InputModalities) != 2 {
		t.Errorf("claude-3 InputModalities: got %v", m.Capabilities.InputModalities)
	}
	if m.Routing == nil || m.Routing.Strategy != "priority" {
		t.Errorf("claude-3 Routing.Strategy: got %v", m.Routing)
	}

	// Top-level routing
	if cfg.Routing == nil || cfg.Routing.Strategy != "priority" {
		t.Errorf("top-level Routing: got %v", cfg.Routing)
	}

	// Model resolution test via config
	resolved, canonical, ok := cfg.ResolveModel("gpt4")
	if !ok || canonical != "gpt-4" || resolved.DisplayName != "GPT-4" {
		t.Errorf("ResolveModel(gpt4): ok=%v canonical=%q display=%q", ok, canonical, resolved.DisplayName)
	}
}

func TestLoadMinimalYAMLDefaults(t *testing.T) {
	yamlData := `
server: {}
providers: {}
models: {}
`
	filePath := "/tmp/test_minimal_config.yaml"
	if err := os.WriteFile(filePath, []byte(yamlData), 0644); err != nil {
		t.Fatalf("failed to write temp config file: %v", err)
	}
	defer os.Remove(filePath)

	cfg, err := Load(filePath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Check defaults
	if cfg.Server.Listen != "127.0.0.1:9000" {
		t.Errorf("default Listen: got %q", cfg.Server.Listen)
	}
	if cfg.Server.LogLevel != "info" {
		t.Errorf("default LogLevel: got %q", cfg.Server.LogLevel)
	}
	if cfg.Server.LogLatencyThresholdMs != 5000 {
		t.Errorf("default LogLatencyThresholdMs: got %d", cfg.Server.LogLatencyThresholdMs)
	}
	if cfg.Server.Session.TTLSeconds != 3600 {
		t.Errorf("default TTLSeconds: got %d", cfg.Server.Session.TTLSeconds)
	}
	if cfg.Server.Session.Backend != "memory" {
		t.Errorf("default Backend: got %q", cfg.Server.Session.Backend)
	}
	if cfg.Server.Session.KeySource != "auto" {
		t.Errorf("default KeySource: got %q", cfg.Server.Session.KeySource)
	}
}

func TestLoadMinimalYAMLWithModelDefaults(t *testing.T) {
	yamlData := `
server: {}
providers:
  p1:
    endpoints:
      chat: "http://p1"
models:
  my-model:
    display_name: "My Model"
`
	filePath := "/tmp/test_minimal_model_config.yaml"
	if err := os.WriteFile(filePath, []byte(yamlData), 0644); err != nil {
		t.Fatalf("failed to write temp config file: %v", err)
	}
	defer os.Remove(filePath)

	cfg, err := Load(filePath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	m, ok := cfg.Models["my-model"]
	if !ok {
		t.Fatal("expected 'my-model' in config")
	}
	if m.Routing == nil {
		t.Fatal("model Routing should have default applied, got nil")
	}
	if m.Routing.Strategy != "priority" {
		t.Errorf("default model routing strategy: got %q", m.Routing.Strategy)
	}
}

func TestProviderAPIKeyReadsFromEnv(t *testing.T) {
	envKey := "TEST_PROVIDER_API_KEY_CONFIG"
	os.Setenv(envKey, "sk-test-key-123")
	defer os.Unsetenv(envKey)

	p := &Provider{APIKeyEnv: envKey}

	cfg := &Config{}
	key := cfg.ProviderAPIKey(p)
	if key != "sk-test-key-123" {
		t.Errorf("expected 'sk-test-key-123', got %q", key)
	}
}

func TestProviderAPIKeyEmptyEnv(t *testing.T) {
	p := &Provider{APIKeyEnv: ""}

	cfg := &Config{}
	key := cfg.ProviderAPIKey(p)
	if key != "" {
		t.Errorf("expected empty string, got %q", key)
	}
}
