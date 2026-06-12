package ingress

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tianyuansun/ai-gateway/pkg/config"
)

func TestSessionCreatedOnFirstRequest(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"ok","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"hi"}}]}`))
	}))
	defer upstream.Close()

	cfg := &config.Config{
		Server: config.ServerConfig{Listen: "127.0.0.1:0"},
		Providers: map[string]config.Provider{
			"p1": {Endpoints: config.ProviderEndpoints{Chat: upstream.URL}},
		},
		Models: map[string]config.Model{
			"m1": {
				Providers: []config.ModelProvider{{Provider: "p1", Priority: 1}},
			},
		},
	}

	gw := NewGateway(cfg)

	body := `{"model":"m1","messages":[{"role":"user","content":"hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	gw.ServeChat(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	sessionID := rec.Header().Get("X-Session-Id")
	if sessionID == "" {
		t.Fatal("expected X-Session-Id header in response, got empty")
	}

	// Verify session exists in the store.
	sess, err := gw.sessions.Get(sessionID)
	if err != nil {
		t.Fatalf("session not stored: %v", err)
	}
	if sess == nil {
		t.Fatal("session not found in store")
	}
	if sess.ProviderID != "p1" {
		t.Errorf("expected session bound to p1, got %s", sess.ProviderID)
	}
}
