package ingress

import (
	"encoding/json"
	"net/http"

	"github.com/tianyuansun/ai-gateway/pkg/config"
	"github.com/tianyuansun/ai-gateway/pkg/provider"
)

type HealthHandler struct {
	cfg     *config.Config
	checker *provider.HealthChecker
}

func NewHealthHandler(cfg *config.Config, checker *provider.HealthChecker) *HealthHandler {
	return &HealthHandler{
		cfg:     cfg,
		checker: checker,
	}
}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	status := "ok"
	providers := make(map[string]string)
	for id := range h.cfg.Providers {
		providers[id] = "healthy"
	}

	// Check if any provider is unhealthy
	for id := range h.checker.Status() {
		if !h.checker.IsHealthy(id) {
			providers[id] = "degraded"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    status,
		"providers": providers,
	})
}
