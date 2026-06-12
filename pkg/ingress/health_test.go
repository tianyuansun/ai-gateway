package ingress

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tianyuansun/ai-gateway/pkg/config"
	"github.com/tianyuansun/ai-gateway/pkg/provider"
)

func TestHealthHandlerReturnsRealStatus(t *testing.T) {
	cfg := &config.Config{
		Providers: map[string]config.Provider{
			"p1": {},
			"p2": {},
		},
	}

	checker := provider.NewHealthChecker(30)
	checker.SetHealth("p1", true)
	checker.SetHealth("p2", false)

	handler := NewHealthHandler(cfg, checker)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	providers, ok := body["providers"].(map[string]interface{})
	if !ok {
		t.Fatal("missing providers in response")
	}

	if providers["p1"] != "healthy" {
		t.Errorf("expected p1=healthy, got %v", providers["p1"])
	}
	if providers["p2"] != "degraded" {
		t.Errorf("expected p2=degraded, got %v", providers["p2"])
	}
}
