package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig          `yaml:"server"`
	Providers map[string]Provider   `yaml:"providers"`
	Models    map[string]Model      `yaml:"models"`
	Routing   *RoutingConfig        `yaml:"routing"`
}

type ServerConfig struct {
	Listen                 string        `yaml:"listen"`
	LogLevel               string        `yaml:"log_level"`
	LogLatencyThresholdMs  int           `yaml:"log_latency_threshold_ms"`
	Session                SessionConfig `yaml:"session"`
}

type SessionConfig struct {
	TTLSeconds int    `yaml:"ttl_seconds"`
	Backend    string `yaml:"backend"`
	KeySource  string `yaml:"key_source"`
	SQLitePath string `yaml:"sqlite_path"`
}

type Provider struct {
	Endpoints ProviderEndpoints `yaml:"endpoints"`
	APIKeyEnv string            `yaml:"api_key_env"`
}

type ProviderEndpoints struct {
	Chat      string `yaml:"chat"`
	Anthropic string `yaml:"anthropic"`
}

type Model struct {
	Aliases      []string         `yaml:"aliases"`
	DisplayName  string           `yaml:"display_name"`
	Capabilities Capabilities     `yaml:"capabilities"`
	Routing      *RoutingConfig   `yaml:"routing"`
	Providers    []ModelProvider  `yaml:"providers"`
}

type Capabilities struct {
	ContextWindow              int      `yaml:"context_window"`
	MaxOutputTokens            int      `yaml:"max_output_tokens"`
	SupportsTools              bool     `yaml:"supports_tools"`
	SupportsParallelToolCalls  bool     `yaml:"supports_parallel_tool_calls"`
	SupportsVision             bool     `yaml:"supports_vision"`
	SupportsReasoning          bool     `yaml:"supports_reasoning"`
	InputModalities            []string `yaml:"input_modalities"`
}

type RoutingConfig struct {
	Strategy string `yaml:"strategy"`
	Affinity string `yaml:"affinity"`
}

type ModelProvider struct {
	Provider string `yaml:"provider"`
	Priority int    `yaml:"priority"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	cfg.applyDefaults()
	return cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Server.Listen == "" {
		c.Server.Listen = "127.0.0.1:9000"
	}
	if c.Server.LogLevel == "" {
		c.Server.LogLevel = "info"
	}
	if c.Server.LogLatencyThresholdMs == 0 {
		c.Server.LogLatencyThresholdMs = 5000
	}
	if c.Server.Session.TTLSeconds == 0 {
		c.Server.Session.TTLSeconds = 3600
	}
	if c.Server.Session.Backend == "" {
		c.Server.Session.Backend = "memory"
	}
	if c.Server.Session.KeySource == "" {
		c.Server.Session.KeySource = "auto"
	}
	if c.Server.Session.SQLitePath == "" {
		c.Server.Session.SQLitePath = "gateway-sessions.db"
	}
	for name, model := range c.Models {
		if model.Routing == nil {
			model.Routing = &RoutingConfig{Strategy: "priority"}
			c.Models[name] = model
		}
	}
}

func (c *Config) ResolveModel(name string) (*Model, string, bool) {
	if m, ok := c.Models[name]; ok {
		return &m, name, true
	}
	for modelName, model := range c.Models {
		for _, alias := range model.Aliases {
			if alias == name {
				return &model, modelName, true
			}
		}
	}
	return nil, "", false
}

func (c *Config) ProviderAPIKey(p *Provider) string {
	if p.APIKeyEnv == "" {
		return ""
	}
	return os.Getenv(p.APIKeyEnv)
}
