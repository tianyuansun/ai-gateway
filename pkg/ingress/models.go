package ingress

import (
	"encoding/json"
	"net/http"

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

type ModelEntry struct {
	ID           string                `json:"id"`
	Object       string                `json:"object"`
	DisplayName  string                `json:"display_name"`
	Aliases      []string              `json:"aliases,omitempty"`
	Capabilities config.Capabilities   `json:"capabilities"`
	Providers    []ProviderEntry       `json:"providers"`
}

type ProviderEntry struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func (h *ModelsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
