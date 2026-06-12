package ingress

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tianyuansun/ai-gateway/pkg/config"
	"github.com/tianyuansun/ai-gateway/pkg/provider"
)

func TestModelsHandlerReturnsRealStatus(t *testing.T) {
	cfg := &config.Config{
		Models: map[string]config.Model{
			"m1": {
				DisplayName: "Model 1",
				Providers: []config.ModelProvider{
					{Provider: "p1"},
					{Provider: "p2"},
				},
			},
		},
	}

	checker := provider.NewHealthChecker(30)
	checker.SetHealth("p1", true)
	checker.SetHealth("p2", false)

	handler := NewModelsHandler(cfg, checker)

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var body struct {
		Data []struct {
			Providers []struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"providers"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(body.Data) != 1 {
		t.Fatalf("expected 1 model, got %d", len(body.Data))
	}

	provs := body.Data[0].Providers
	if len(provs) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(provs))
	}
	if provs[0].Status != "healthy" {
		t.Errorf("expected p1=healthy, got %s", provs[0].Status)
	}
	if provs[1].Status != "degraded" {
		t.Errorf("expected p2=degraded, got %s", provs[1].Status)
	}
}
