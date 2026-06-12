package ingress

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tianyuansun/ai-gateway/pkg/config"
	"github.com/tianyuansun/ai-gateway/pkg/provider"
)

func TestModelsHandlerReturnsAnthropicFormat(t *testing.T) {
	cfg := &config.Config{
		Models: map[string]config.Model{
			"m1": {
				DisplayName: "Model 1",
				Capabilities: config.Capabilities{
					ContextWindow:             262144,
					MaxOutputTokens:           16384,
					SupportsTools:             true,
					SupportsReasoning:         false,
					SupportsVision:            false,
				},
				Providers: []config.ModelProvider{
					{Provider: "p1"},
				},
			},
		},
	}

	checker := provider.NewHealthChecker(30)
	handler := NewModelsHandler(cfg, checker)

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("x-api-key", "sk-test")
	req.Header.Set("anthropic-version", "2023-06-01")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body struct {
		Data    []map[string]any `json:"data"`
		HasMore bool             `json:"has_more"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(body.Data) != 1 {
		t.Fatalf("expected 1 model, got %d", len(body.Data))
	}

	m := body.Data[0]
	if m["type"] != "model" {
		t.Errorf("expected type=model, got %v", m["type"])
	}
	if m["id"] != "m1" {
		t.Errorf("expected id=m1, got %v", m["id"])
	}

	// Capabilities should be nested objects
	caps, ok := m["capabilities"].(map[string]any)
	if !ok {
		t.Fatal("capabilities should be a map")
	}
	if tools, ok := caps["tools"].(map[string]any); ok {
		if tools["supported"] != true {
			t.Errorf("tools.supported should be true")
		}
	}
}

func TestModelsHandlerReturnsOpenAIFormatDefault(t *testing.T) {
	cfg := &config.Config{
		Models: map[string]config.Model{
			"m1": {
				DisplayName: "Model 1",
				Providers:   []config.ModelProvider{{Provider: "p1"}},
			},
		},
	}

	handler := NewModelsHandler(cfg, nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var body struct {
		Object string `json:"object"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Object != "list" {
		t.Errorf("default format should be OpenAI (object=list), got %q", body.Object)
	}
}

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
