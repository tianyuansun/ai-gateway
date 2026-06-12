package ingress

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tianyuansun/ai-gateway/pkg/config"
)

func TestE2E_RoutesToHealthyProvider(t *testing.T) {
	healthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"id":      "healthy-response",
			"object":  "chat.completion",
			"choices": []map[string]any{{"index": float64(0), "message": map[string]any{"role": "assistant", "content": "from healthy"}}},
		})
	}))
	defer healthyServer.Close()

	unhealthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer unhealthyServer.Close()

	cfg := &config.Config{
		Server: config.ServerConfig{Listen: "127.0.0.1:0"},
		Providers: map[string]config.Provider{
			"p-unhealthy": {Endpoints: config.ProviderEndpoints{Chat: unhealthyServer.URL}},
			"p-healthy":   {Endpoints: config.ProviderEndpoints{Chat: healthyServer.URL}},
		},
		Models: map[string]config.Model{
			"test-model": {
				Routing: &config.RoutingConfig{Strategy: "priority"},
				Providers: []config.ModelProvider{
					{Provider: "p-unhealthy", Priority: 1},
					{Provider: "p-healthy", Priority: 2},
				},
			},
		},
	}

	gw := NewGateway(cfg)
	gw.health.SetHealth("p-unhealthy", false)
	// p-healthy defaults to healthy, no SetHealth needed

	body := `{"model":"test-model","messages":[{"role":"user","content":"hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	gw.ServeChat(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.ID != "healthy-response" {
		t.Errorf("expected response from healthy provider, got id=%q", resp.ID)
	}
}
