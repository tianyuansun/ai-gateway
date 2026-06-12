package ingress

import (
	"fmt"
	"net/http"

	"github.com/tianyuansun/ai-gateway/pkg/config"
)

type CodexConfigHandler struct {
	cfg *config.Config
}

func NewCodexConfigHandler(cfg *config.Config) *CodexConfigHandler {
	return &CodexConfigHandler{cfg: cfg}
}

type codexConfigTOML struct {
	Model          string                          `toml:"model"`
	ModelProvider  string                          `toml:"model_provider"`
	Providers      map[string]codexProviderSection `toml:"model_providers"`
	Properties     map[string]codexProperties      `toml:"model_properties"`
}

type codexProviderSection struct {
	Name    string `toml:"name"`
	BaseURL string `toml:"base_url"`
	WireAPI string `toml:"wire_api"`
	EnvKey  string `toml:"env_key"`
}

type codexProperties struct {
	ContextWindow             int      `toml:"context_window"`
	MaxContextWindow          int      `toml:"max_context_window"`
	SupportsParallelToolCalls bool     `toml:"supports_parallel_tool_calls"`
	SupportsReasoningSummaries bool    `toml:"supports_reasoning_summaries"`
	InputModalities           []string `toml:"input_modalities"`
}

func (h *CodexConfigHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	modelName := r.URL.Query().Get("model")
	if modelName == "" {
		modelName = "deepseek-v4-pro"
	}

	model, _, ok := h.cfg.ResolveModel(modelName)
	if !ok {
		http.Error(w, "model not found", http.StatusNotFound)
		return
	}

	cfg := codexConfigTOML{
		Model:         modelName,
		ModelProvider: "ai-gateway",
		Providers: map[string]codexProviderSection{
			"ai-gateway": {
				Name:    "AI Gateway",
				BaseURL: "http://" + h.cfg.Server.Listen + "/v1",
				WireAPI: "responses",
				EnvKey:  "GATEWAY_API_KEY",
			},
		},
		Properties: map[string]codexProperties{
			modelName: {
				ContextWindow:              model.Capabilities.ContextWindow,
				MaxContextWindow:           model.Capabilities.ContextWindow * 4,
				SupportsParallelToolCalls:  model.Capabilities.SupportsParallelToolCalls,
				SupportsReasoningSummaries: false,
				InputModalities:            model.Capabilities.InputModalities,
			},
		},
	}

	// Output as TOML-like text (simple approach, we can add TOML encoder later)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	writeTOML(w, cfg)
}

func writeTOML(w http.ResponseWriter, cfg codexConfigTOML) {
	w.Write([]byte("model = \"" + cfg.Model + "\"\n"))
	w.Write([]byte("model_provider = \"" + cfg.ModelProvider + "\"\n\n"))
	for key, prov := range cfg.Providers {
		w.Write([]byte("[model_providers.\"" + key + "\"]\n"))
		w.Write([]byte("name = \"" + prov.Name + "\"\n"))
		w.Write([]byte("base_url = \"" + prov.BaseURL + "\"\n"))
		w.Write([]byte("wire_api = \"" + prov.WireAPI + "\"\n"))
		w.Write([]byte("env_key = \"" + prov.EnvKey + "\"\n\n"))
	}
	for key, props := range cfg.Properties {
		w.Write([]byte("[model_properties.\"" + key + "\"]\n"))
		w.Write([]byte("context_window = " + itoa(props.ContextWindow) + "\n"))
		w.Write([]byte("max_context_window = " + itoa(props.MaxContextWindow) + "\n"))
		w.Write([]byte("supports_parallel_tool_calls = " + boolStr(props.SupportsParallelToolCalls) + "\n"))
		w.Write([]byte("supports_reasoning_summaries = " + boolStr(props.SupportsReasoningSummaries) + "\n"))
		w.Write([]byte("input_modalities = [\"" + join(props.InputModalities, "\", \"") + "\"]\n"))
	}
}

func itoa(n int) string { return itoaStr(n) }
func itoaStr(n int) string { return fmt.Sprintf("%d", n) }
func boolStr(b bool) string { if b { return "true" }; return "false" }
func join(s []string, sep string) string {
	if len(s) == 0 { return "" }
	result := s[0]
	for i := 1; i < len(s); i++ {
		result += sep + s[i]
	}
	return result
}
