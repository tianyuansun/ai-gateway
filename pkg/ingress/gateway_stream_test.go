package ingress

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tianyuansun/ai-gateway/pkg/config"
)

func TestGatewayStreamingResponse(t *testing.T) {
	// Upstream server that returns SSE chunks.
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify upstream received stream:true
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		chunks := []string{
			`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"delta":{"role":"assistant"},"index":0}]}`,
			`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"delta":{"content":"Hello"},"index":0}]}`,
			`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"delta":{},"finish_reason":"stop","index":0}]}`,
			`data: [DONE]`,
		}
		for _, chunk := range chunks {
			w.Write([]byte(chunk + "\n\n"))
			flusher.Flush()
		}
	}))
	defer upstreamServer.Close()

	cfg := &config.Config{
		Server: config.ServerConfig{Listen: "127.0.0.1:0"},
		Providers: map[string]config.Provider{
			"p1": {Endpoints: config.ProviderEndpoints{Chat: upstreamServer.URL}},
		},
		Models: map[string]config.Model{
			"test-model": {
				Routing: &config.RoutingConfig{Strategy: "priority"},
				Providers: []config.ModelProvider{
					{Provider: "p1", Priority: 1},
				},
			},
		},
	}

	gw := NewGateway(cfg)
	// Mark p1 as healthy.
	gw.health.SetHealth("p1", true)

	body := `{"model":"test-model","messages":[{"role":"user","content":"hello"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	gw.ServeChat(rec, req)

	// Verify SSE content type.
	ct := rec.Header().Get("Content-Type")
	if ct != "text/event-stream" {
		t.Fatalf("expected Content-Type text/event-stream, got %q", ct)
	}

	// Parse SSE events from the response.
	scanner := bufio.NewScanner(rec.Body)
	var events []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			events = append(events, line)
		}
	}

	if len(events) < 2 {
		t.Fatalf("expected at least 2 SSE data events, got %d: %v", len(events), events)
	}
}

func TestGatewayStreamingResponsesAPI(t *testing.T) {
	// Upstream Anthropic endpoint returning SSE.
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		chunks := []string{
			`event: message_start`,
			`data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude","usage":{"input_tokens":5}}}`,
			``,
			`event: content_block_delta`,
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hi"}}`,
			``,
			`event: message_stop`,
			`data: {"type":"message_stop"}`,
			``,
		}
		for _, chunk := range chunks {
			w.Write([]byte(chunk + "\n"))
			flusher.Flush()
		}
	}))
	defer upstreamServer.Close()

	cfg := &config.Config{
		Server: config.ServerConfig{Listen: "127.0.0.1:0"},
		Providers: map[string]config.Provider{
			"p1": {Endpoints: config.ProviderEndpoints{Anthropic: upstreamServer.URL}},
		},
		Models: map[string]config.Model{
			"test-model": {
				Routing: &config.RoutingConfig{Strategy: "priority"},
				Providers: []config.ModelProvider{
					{Provider: "p1", Priority: 1},
				},
			},
		},
	}

	gw := NewGateway(cfg)
	gw.health.SetHealth("p1", true)

	body := `{"model":"test-model","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	gw.ServeResponses(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "text/event-stream" {
		t.Fatalf("expected Content-Type text/event-stream, got %q", ct)
	}

	// Verify we get response.created and response.completed events.
	bodyStr := rec.Body.String()
	if !strings.Contains(bodyStr, "response.created") {
		t.Error("expected 'response.created' event")
	}
	if !strings.Contains(bodyStr, "response.completed") {
		t.Error("expected 'response.completed' event")
	}
}

func TestGatewayNonStreamingStillWorks(t *testing.T) {
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"resp-1","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"hello back"}}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`))
	}))
	defer upstreamServer.Close()

	cfg := &config.Config{
		Server: config.ServerConfig{Listen: "127.0.0.1:0"},
		Providers: map[string]config.Provider{
			"p1": {Endpoints: config.ProviderEndpoints{Chat: upstreamServer.URL}},
		},
		Models: map[string]config.Model{
			"test-model": {
				Routing: &config.RoutingConfig{Strategy: "priority"},
				Providers: []config.ModelProvider{
					{Provider: "p1", Priority: 1},
				},
			},
		},
	}

	gw := NewGateway(cfg)
	gw.health.SetHealth("p1", true)

	body := `{"model":"test-model","messages":[{"role":"user","content":"hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	gw.ServeChat(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %s", ct)
	}
}
