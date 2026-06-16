package ingress

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tianyuansun/ai-gateway/pkg/config"
	chat "github.com/tianyuansun/ai-gateway/pkg/schema/chat"
)

func TestE2E_RoutesToHealthyProvider(t *testing.T) {
	healthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return SSE (streaming) since the gateway sends stream:true upstream.
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		respData, _ := json.Marshal(map[string]any{
			"id":      "healthy-response",
			"object":  "chat.completion",
			"choices": []map[string]any{{"index": float64(0), "message": map[string]any{"role": "assistant", "content": "from healthy"}}},
		})
		chunks := []string{
			`data: {"id":"healthy-response","object":"chat.completion.chunk","choices":[{"delta":{"content":"from healthy"},"index":0}]}`,
			`data: [DONE]`,
		}
		for _, chunk := range chunks {
			w.Write([]byte(chunk + "\n\n"))
			if flusher != nil {
				flusher.Flush()
			}
		}
		_ = respData
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

func TestE2E_ChatCompletionsNonStreaming(t *testing.T) {
	// Mock upstream returns Chat Completion JSON.
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"chatcmpl-1","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"Hello from AI"}}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`))
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

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	var respBody map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &respBody); err != nil {
		t.Fatalf("invalid response JSON: %v", err)
	}
	if _, ok := respBody["object"]; !ok {
		t.Error("response body missing 'object' field")
	}
}

func TestE2E_AnthropicMessagesNonStreaming(t *testing.T) {
	// Mock upstream returns Anthropic Message JSON.
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-4-20250514","content":[{"type":"text","text":"Hello from Claude"}],"usage":{"input_tokens":10,"output_tokens":5}}`))
	}))
	defer upstreamServer.Close()

	cfg := &config.Config{
		Server: config.ServerConfig{Listen: "127.0.0.1:0"},
		Providers: map[string]config.Provider{
			"p1": {Endpoints: config.ProviderEndpoints{Anthropic: upstreamServer.URL}},
		},
		Models: map[string]config.Model{
			"claude-sonnet-4-20250514": {
				Routing: &config.RoutingConfig{Strategy: "priority"},
				Providers: []config.ModelProvider{
					{Provider: "p1", Priority: 1},
				},
			},
		},
	}

	gw := NewGateway(cfg)
	gw.health.SetHealth("p1", true)

	body := `{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"hello"}],"max_tokens":100}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	gw.ServeMessages(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var respBody map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &respBody); err != nil {
		t.Fatalf("invalid response JSON: %v", err)
	}
	if _, ok := respBody["object"]; !ok {
		t.Error("response body missing 'object' field")
	}
}

func TestE2E_ResponsesAPINonStreaming(t *testing.T) {
	// Mock upstream returns Anthropic SSE (ResToAnth translator forces stream:true).
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

	body := `{"model":"test-model","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	gw.ServeResponses(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var respBody map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &respBody); err != nil {
		t.Fatalf("invalid response JSON: %v", err)
	}

	obj, ok := respBody["object"].(string)
	if !ok || obj != "response" {
		t.Errorf("expected object='response', got %v", respBody["object"])
	}
}

func TestE2E_AnthToResTranslatorRegistered(t *testing.T) {
	// Set up an upstream server that handles Responses API requests and returns
	// Responses API SSE events.
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		chunks := []string{
			`data: {"type":"response.created","response":{"id":"resp_1","object":"response"}}`,
			`data: {"type":"response.in_progress","response":{"id":"resp_1","object":"response"}}`,
			`data: {"type":"response.output_item.added","output_index":0,"item":{"id":"resp_1_item","type":"message","role":"assistant","status":"in_progress"}}`,
			`data: {"type":"response.content_part.added","item_id":"resp_1_item","output_index":0,"content_index":0,"part":{"type":"output_text","text":""}}`,
			`data: {"type":"response.output_text.delta","item_id":"resp_1_item","output_index":0,"content_index":0,"delta":"Hello from responses"}`,
			`data: {"type":"response.content_part.done","item_id":"resp_1_item","output_index":0,"content_index":0,"part":{"type":"output_text","text":"Hello from responses"}}`,
			`data: {"type":"response.output_item.done","output_index":0,"item":{"id":"resp_1_item","type":"message","role":"assistant","status":"completed"}}`,
			`data: {"type":"response.completed","response":{"id":"resp_1","object":"response"}}`,
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
			"p-responses": {Endpoints: config.ProviderEndpoints{Responses: upstreamServer.URL}},
		},
		Models: map[string]config.Model{
			"test-model": {
				Routing: &config.RoutingConfig{Strategy: "priority"},
				Providers: []config.ModelProvider{
					{Provider: "p-responses", Priority: 1},
				},
			},
		},
	}

	gw := NewGateway(cfg)
	gw.health.SetHealth("p-responses", true)

	// Send an Anthropic Messages API request. The gateway should use the
	// AnthToRes translator since the provider has only a Responses endpoint.
	body := `{"model":"test-model","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"max_tokens":100,"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	gw.ServeMessages(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify SSE content type.
	ct := rec.Header().Get("Content-Type")
	if ct != "text/event-stream" {
		t.Fatalf("expected text/event-stream, got %s", ct)
	}

	// Verify we get Anthropic-style SSE events (message_start, content_block_delta, etc.).
	bodyStr := rec.Body.String()
	if !strings.Contains(bodyStr, "message_start") {
		t.Error("expected message_start in response")
	}
	if !strings.Contains(bodyStr, "content_block_delta") {
		t.Error("expected content_block_delta in response")
	}
	if !strings.Contains(bodyStr, "message_stop") {
		t.Error("expected message_stop in response")
	}
	if !strings.Contains(bodyStr, "Hello from responses") {
		t.Error("expected 'Hello from responses' in response body")
	}
}

func TestE2E_ResponsesAPI_InstructionsForwardedAsSystemMessage(t *testing.T) {
	var upstreamBody []byte

	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		upstreamBody = body
		r.Body.Close()

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		chunks := []string{
			`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"delta":{"content":"Handoff summary: task complete."},"index":0}]}`,
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
	gw.health.SetHealth("p1", true)

	body := `{"model":"test-model","instructions":"You are a summarizer. Produce a handoff summary.","input":[{"type":"message","role":"user","content":"hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	gw.ServeResponses(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var chatReq chat.ChatCompletionRequest
	if err := json.Unmarshal(upstreamBody, &chatReq); err != nil {
		t.Fatalf("failed to unmarshal upstream body as Chat request: %v\nBody: %s", err, string(upstreamBody))
	}

	if len(chatReq.Messages) < 2 {
		t.Fatalf("expected at least 2 messages (system + user), got %d", len(chatReq.Messages))
	}

	msg0 := chatReq.Messages[0]
	if msg0.Role != "system" {
		t.Errorf("expected first message role 'system', got %q", msg0.Role)
	}
	if msg0.Content == nil || msg0.Content.String == nil {
		t.Fatal("expected first message to have string content")
	}
	if *msg0.Content.String != "You are a summarizer. Produce a handoff summary." {
		t.Errorf("expected instructions text, got %q", *msg0.Content.String)
	}
}

func TestE2E_ResponsesAPI_NoInstructions_NoSystemMessageUpstream(t *testing.T) {
	var upstreamBody []byte

	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		upstreamBody = body
		r.Body.Close()

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		w.Write([]byte(`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"delta":{"content":"ok"},"index":0}]}` + "\n\n"))
		flusher.Flush()
		w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
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

	body := `{"model":"test-model","input":[{"type":"message","role":"user","content":"hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	gw.ServeResponses(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var chatReq chat.ChatCompletionRequest
	if err := json.Unmarshal(upstreamBody, &chatReq); err != nil {
		t.Fatalf("failed to unmarshal upstream body: %v", err)
	}

	for _, msg := range chatReq.Messages {
		if msg.Role == "system" {
			t.Error("expected no system message when instructions is absent")
		}
	}
}

func TestE2E_ResponsesAPI_CompactSimulation(t *testing.T) {
	var upstreamBody []byte

	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		upstreamBody = body
		r.Body.Close()

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		chunks := []string{
			`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"delta":{"content":"summary: project is a REST API"},"index":0}]}`,
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
	gw.health.SetHealth("p1", true)

	// Simulate a local compact request: instructions + multi-item conversation input.
	body := `{"model":"test-model","instructions":"You are performing a CONTEXT CHECKPOINT COMPACTION. Create a handoff summary.","input":[{"type":"message","role":"user","content":"write a REST API"},{"type":"message","role":"assistant","content":"I will build it with Go."},{"type":"message","role":"user","content":"add tests"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	gw.ServeResponses(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var chatReq chat.ChatCompletionRequest
	if err := json.Unmarshal(upstreamBody, &chatReq); err != nil {
		t.Fatalf("failed to unmarshal upstream body: %v", err)
	}

	if len(chatReq.Messages) < 4 {
		t.Fatalf("expected at least 4 messages (system + 3 conversation), got %d", len(chatReq.Messages))
	}

	msg0 := chatReq.Messages[0]
	if msg0.Role != "system" {
		t.Errorf("expected first message role 'system', got %q", msg0.Role)
	}
	if msg0.Content == nil || msg0.Content.String == nil {
		t.Fatal("expected first message to have string content")
	}
	if !strings.Contains(*msg0.Content.String, "CONTEXT CHECKPOINT COMPACTION") {
		t.Errorf("expected summarization prompt in system message, got %q", *msg0.Content.String)
	}

	// Conversation messages should follow in order.
	if chatReq.Messages[1].Role != "user" {
		t.Errorf("expected msg[1] role 'user', got %q", chatReq.Messages[1].Role)
	}
	if chatReq.Messages[2].Role != "assistant" {
		t.Errorf("expected msg[2] role 'assistant', got %q", chatReq.Messages[2].Role)
	}
	if chatReq.Messages[3].Role != "user" {
		t.Errorf("expected msg[3] role 'user', got %q", chatReq.Messages[3].Role)
	}
}

func TestE2E_ResponsesAPI_ReasoningStream(t *testing.T) {
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		chunks := []string{
			`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"delta":{"reasoning_content":"Let me think."},"index":0}]}`,
			`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"delta":{"content":"Answer"},"index":0}]}`,
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
	gw.health.SetHealth("p1", true)

	body := `{"model":"test-model","stream":true,"input":[{"type":"message","role":"user","content":"think hard"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	gw.ServeResponses(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), "reasoning_summary_text.delta") {
		t.Error("expected reasoning_summary_text.delta in response")
	}
	if !strings.Contains(rec.Body.String(), "Let me think.") {
		t.Error("expected reasoning text in response")
	}
	if !strings.Contains(rec.Body.String(), "output_text.delta") {
		t.Error("expected output_text.delta in response")
	}
}
