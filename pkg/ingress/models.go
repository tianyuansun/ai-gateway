package ingress

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/tianyuansun/ai-gateway/pkg/config"
	"github.com/tianyuansun/ai-gateway/pkg/provider"
)

type ModelsHandler struct {
	cfg     *config.Config
	checker *provider.HealthChecker
}

func NewModelsHandler(cfg *config.Config, checker *provider.HealthChecker) *ModelsHandler {
	return &ModelsHandler{cfg: cfg, checker: checker}
}

// ModelEntry is the OpenAI-format model entry.
type ModelEntry struct {
	ID           string              `json:"id"`
	Object       string              `json:"object"`
	DisplayName  string              `json:"display_name"`
	Aliases      []string            `json:"aliases,omitempty"`
	Capabilities config.Capabilities `json:"capabilities"`
	Providers    []ProviderEntry     `json:"providers"`
}

type ProviderEntry struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func (h *ModelsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.useAnthropicFormat(r) {
		h.serveAnthropic(w)
	} else {
		h.serveOpenAI(w)
	}
}

// useAnthropicFormat detects whether the client expects Anthropic format.
func (h *ModelsHandler) useAnthropicFormat(r *http.Request) bool {
	// Priority 1: Accept header explicitly requests Anthropic.
	if strings.Contains(r.Header.Get("Accept"), "x-anthropic-json") {
		return true
	}
	// Priority 2: Anthropic auth headers present.
	if r.Header.Get("x-api-key") != "" && r.Header.Get("anthropic-version") != "" {
		return true
	}
	return false
}

func (h *ModelsHandler) serveAnthropic(w http.ResponseWriter) {
	type anthCap struct {
		Supported bool `json:"supported"`
	}
	type anthThinkingCaps struct {
		Supported bool                   `json:"supported"`
		Types     map[string]anthCap     `json:"types"`
	}
	type anthModel struct {
		Type           string           `json:"type"`
		ID             string           `json:"id"`
		DisplayName    string           `json:"display_name"`
		CreatedAt      string           `json:"created_at"`
		MaxInputTokens int              `json:"max_input_tokens"`
		MaxTokens      int              `json:"max_tokens"`
		Capabilities   map[string]any   `json:"capabilities"`
	}

	var data []anthModel
	for name, model := range h.cfg.Models {
		caps := map[string]any{
			"batch":              anthCap{Supported: false},
			"citations":          anthCap{Supported: false},
			"code_execution":     anthCap{Supported: false},
			"image_input":        anthCap{Supported: model.Capabilities.SupportsVision},
			"pdf_input":          anthCap{Supported: false},
			"structured_outputs": anthCap{Supported: false},
			"tools":              anthCap{Supported: model.Capabilities.SupportsTools},
			"thinking": anthThinkingCaps{
				Supported: model.Capabilities.SupportsReasoning,
				Types: map[string]anthCap{
					"enabled":  {Supported: model.Capabilities.SupportsReasoning},
					"adaptive": {Supported: model.Capabilities.SupportsReasoning},
				},
			},
			"effort": map[string]any{
				"supported": model.Capabilities.SupportsReasoning,
				"low":       anthCap{Supported: model.Capabilities.SupportsReasoning},
				"medium":    anthCap{Supported: model.Capabilities.SupportsReasoning},
				"high":      anthCap{Supported: model.Capabilities.SupportsReasoning},
			},
		}

		data = append(data, anthModel{
			Type:           "model",
			ID:             name,
			DisplayName:    model.DisplayName,
			CreatedAt:      "2026-01-01T00:00:00Z",
			MaxInputTokens: model.Capabilities.ContextWindow,
			MaxTokens:      model.Capabilities.MaxOutputTokens,
			Capabilities:   caps,
		})
	}

	firstID, lastID := "", ""
	if len(data) > 0 {
		firstID = data[0].ID
		lastID = data[len(data)-1].ID
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data":     data,
		"has_more": false,
		"first_id": firstID,
		"last_id":  lastID,
	})
}

func (h *ModelsHandler) serveOpenAI(w http.ResponseWriter) {
	data := make([]ModelEntry, 0, len(h.cfg.Models))
	for name, model := range h.cfg.Models {
		entry := ModelEntry{
			ID:           name,
			Object:       "model",
			DisplayName:  model.DisplayName,
			Aliases:      model.Aliases,
			Capabilities: model.Capabilities,
			Providers:    make([]ProviderEntry, len(model.Providers)),
		}
		for i, mp := range model.Providers {
			status := "healthy"
			if h.checker != nil && !h.checker.IsHealthy(mp.Provider) {
				status = "degraded"
			}
			entry.Providers[i] = ProviderEntry{ID: mp.Provider, Status: status}
		}
		data = append(data, entry)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"object": "list",
		"data":   data,
	})
}
