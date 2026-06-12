package translator

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tianyuansun/ai-gateway/pkg/config"
	"github.com/tianyuansun/ai-gateway/pkg/provider"
	"github.com/tianyuansun/ai-gateway/pkg/router"
	"github.com/tianyuansun/ai-gateway/pkg/schema/anthropic"
	"github.com/tianyuansun/ai-gateway/pkg/schema/chat"
	"github.com/tianyuansun/ai-gateway/pkg/schema/responses"
	"github.com/tianyuansun/ai-gateway/pkg/session"
)

// TestResToAnth_Integration_RoundTrip wires translator + router + session +
// provider together manually and verifies the complete request/response
// roundtrip for the Responses→Anthropic path.
func TestResToAnth_Integration_RoundTrip(t *testing.T) {
	// 1. Mock upstream that returns an Anthropic Messages API JSON response.
	var capturedBody []byte
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		capturedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("mock server read body: %v", err)
		}
		r.Body.Close()

		anthResp := anthropic.MessageResponse{
			ID:    "msg_integ_1",
			Type:  "message",
			Role:  "assistant",
			Model: "claude-sonnet-4-20250514",
			Content: []anthropic.ResponseContentBlock{
				{Type: "text", Text: "Hello from Anthropic integration test"},
			},
			Usage: anthropic.Usage{InputTokens: 10, OutputTokens: 5},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(anthResp)
	}))
	defer mockServer.Close()

	// 2. Config: one model → one provider pointing at the mock server.
	cfg := &config.Config{
		Providers: map[string]config.Provider{
			"mock-prov": {
				Endpoints: config.ProviderEndpoints{Anthropic: mockServer.URL},
			},
		},
		Models: map[string]config.Model{
			"test-model": {
				Providers: []config.ModelProvider{
					{Provider: "mock-prov", Priority: 1},
				},
			},
		},
	}

	// 3. Session store, health checker.
	sessStore := session.NewMemoryStore(100, 3600)
	health := provider.NewHealthChecker(30)
	health.SetHealth("mock-prov", true)

	// 4. Router components.
	resolver := router.NewModelResolver(cfg)
	selector := router.NewProviderSelector(cfg, sessStore, health)

	// 5. Translator and upstream client.
	tr := &ResToAnth{}
	client := provider.NewClient(30 * time.Second)

	// 6. Resolve model via ModelResolver.
	model, canonicalName, ok := resolver.Resolve("test-model")
	if !ok {
		t.Fatal("model not resolved")
	}
	if canonicalName != "test-model" {
		t.Errorf("canonical name mismatch: got %q, want %q", canonicalName, "test-model")
	}

	// 7. Create a session.
	sess := &session.Session{ID: "sess-integ-1"}

	// 8. Select provider via ProviderSelector.
	prov, provID, err := selector.Select(model, sess.ID)
	if err != nil {
		t.Fatalf("provider selection: %v", err)
	}
	// Bind provider to session (as the Gateway would).
	sess.ProviderID = provID
	sess.ModelName = canonicalName

	// 9. Build a Responses API request body.
	reqBody := buildResponsesRequest(t, "test-model", "Hello, how are you?")
	tReq := &Request{
		Model:     canonicalName,
		APIFormat: FormatResponses,
		Body:      reqBody,
	}

	// 10. Translate request via ResToAnth.TranslateRequest.
	upReq, err := tr.TranslateRequest(context.Background(), tReq, sess)
	if err != nil {
		t.Fatalf("TranslateRequest: %v", err)
	}

	// 11. Verify upstream request has correct Anthropic format.
	if upReq.URL != "/messages" {
		t.Errorf("expected upstream URL /messages, got %q", upReq.URL)
	}
	var anthReq anthropic.MessageRequest
	if err := json.Unmarshal(upReq.Body, &anthReq); err != nil {
		t.Fatalf("unmarshal upstream request body: %v", err)
	}
	if anthReq.Model != "test-model" {
		t.Errorf("expected model test-model in upstream request, got %q", anthReq.Model)
	}
	if !anthReq.Stream {
		t.Error("expected stream=true in upstream request")
	}
	if len(anthReq.Messages) == 0 {
		t.Error("expected at least one message in upstream request")
	}

	// 12. Call upstream via provider.Client.Call.
	httpResp, err := client.Call(context.Background(), prov.Endpoints.Anthropic, upReq.URL, "", upReq.Body, upReq.Headers)
	if err != nil {
		t.Fatalf("upstream call: %v", err)
	}

	// 13. Verify the upstream received the correct request body.
	var sentReq anthropic.MessageRequest
	if err := json.Unmarshal(capturedBody, &sentReq); err != nil {
		t.Fatalf("unmarshal captured body: %v", err)
	}
	if sentReq.Model != "test-model" {
		t.Errorf("mock upstream received model %q, expected test-model", sentReq.Model)
	}
	if len(sentReq.Messages) != 1 {
		t.Errorf("mock upstream received %d messages, expected 1", len(sentReq.Messages))
	}

	// 14. Translate response via ResToAnth.TranslateResponse.
	resp, err := tr.TranslateResponse(context.Background(), httpResp, tReq, sess)
	if err != nil {
		t.Fatalf("TranslateResponse: %v", err)
	}

	// 15. Verify response is translated to correct Responses API format.
	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	var responsesResp responses.Response
	if err := json.Unmarshal(resp.Body, &responsesResp); err != nil {
		t.Fatalf("unmarshal translated response: %v", err)
	}
	if responsesResp.ID != "msg_integ_1" {
		t.Errorf("expected ID msg_integ_1, got %q", responsesResp.ID)
	}
	if responsesResp.Object != "response" {
		t.Errorf("expected object 'response', got %q", responsesResp.Object)
	}
	if len(responsesResp.Output) == 0 {
		t.Error("expected at least one output item")
	} else {
		out0 := responsesResp.Output[0]
		if out0.Type != "message" {
			t.Errorf("expected output[0].type 'message', got %q", out0.Type)
		}
		if out0.Role != "assistant" {
			t.Errorf("expected output[0].role 'assistant', got %q", out0.Role)
		}
		if len(out0.Content) == 0 || out0.Content[0].Text != "Hello from Anthropic integration test" {
			t.Errorf("unexpected output content: %+v", out0.Content)
		}
	}
	if responsesResp.Usage == nil {
		t.Error("expected usage in response")
	} else if responsesResp.Usage.TotalTokens != 15 {
		t.Errorf("expected total_tokens 15, got %d", responsesResp.Usage.TotalTokens)
	}

	// 16. Update session (reasoning records, etc.).
	tr.UpdateSession(sess, tReq, resp)

	// 17. Verify session state: provider bound.
	if sess.ProviderID != "mock-prov" {
		t.Errorf("expected session provider mock-prov, got %q", sess.ProviderID)
	}
	if sess.ModelName != "test-model" {
		t.Errorf("expected session model test-model, got %q", sess.ModelName)
	}
}

// TestResToChat_Integration_RoundTrip wires translator + router + session +
// provider together manually for the Responses→Chat Completions path.
func TestResToChat_Integration_RoundTrip(t *testing.T) {
	// 1. Mock upstream that returns a Chat Completions JSON response.
	var capturedBody []byte
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		capturedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("mock server read body: %v", err)
		}
		r.Body.Close()

		chatResp := chat.ChatCompletion{
			ID:     "chatcmpl_integ_1",
			Object: "chat.completion",
			Model:  "gpt-5",
			Choices: []chat.ChatCompletionChoice{
				{
					Index: 0,
					Message: chat.ChatCompletionMessage{
						Role:    "assistant",
						Content: &chat.ChatCompletionMessageContent{String: strPtr("Hello from Chat integration test")},
					},
					FinishReason: strPtr("stop"),
				},
			},
			Usage: &chat.CompletionUsage{PromptTokens: 8, CompletionTokens: 4, TotalTokens: 12},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResp)
	}))
	defer mockServer.Close()

	// 2. Config: one model → one provider pointing at the mock server.
	cfg := &config.Config{
		Providers: map[string]config.Provider{
			"mock-prov": {
				Endpoints: config.ProviderEndpoints{Chat: mockServer.URL},
			},
		},
		Models: map[string]config.Model{
			"test-model": {
				Providers: []config.ModelProvider{
					{Provider: "mock-prov", Priority: 1},
				},
			},
		},
	}

	// 3. Session store, health checker.
	sessStore := session.NewMemoryStore(100, 3600)
	health := provider.NewHealthChecker(30)
	health.SetHealth("mock-prov", true)

	// 4. Router components.
	resolver := router.NewModelResolver(cfg)
	selector := router.NewProviderSelector(cfg, sessStore, health)

	// 5. Translator and upstream client.
	tr := &ResToChat{}
	client := provider.NewClient(30 * time.Second)

	// 6. Resolve model.
	model, canonicalName, ok := resolver.Resolve("test-model")
	if !ok {
		t.Fatal("model not resolved")
	}

	// 7. Create a session.
	sess := &session.Session{ID: "sess-integ-2"}

	// 8. Select provider.
	prov, provID, err := selector.Select(model, sess.ID)
	if err != nil {
		t.Fatalf("provider selection: %v", err)
	}
	sess.ProviderID = provID
	sess.ModelName = canonicalName

	// 9. Build a Responses API request body.
	reqBody := buildResponsesRequest(t, "test-model", "What is the meaning of life?")
	tReq := &Request{
		Model:     canonicalName,
		APIFormat: FormatResponses,
		Body:      reqBody,
	}

	// 10. Translate request via ResToChat.TranslateRequest.
	upReq, err := tr.TranslateRequest(context.Background(), tReq, sess)
	if err != nil {
		t.Fatalf("TranslateRequest: %v", err)
	}

	// 11. Verify upstream request has correct Chat Completions format.
	if upReq.URL != "/chat/completions" {
		t.Errorf("expected upstream URL /chat/completions, got %q", upReq.URL)
	}
	var chatReq chat.ChatCompletionRequest
	if err := json.Unmarshal(upReq.Body, &chatReq); err != nil {
		t.Fatalf("unmarshal upstream request body: %v", err)
	}
	if chatReq.Model != "test-model" {
		t.Errorf("expected model test-model in upstream request, got %q", chatReq.Model)
	}
	if !chatReq.Stream {
		t.Error("expected stream=true in upstream request")
	}
	if len(chatReq.Messages) == 0 {
		t.Error("expected at least one message in upstream request")
	}

	// 12. Call upstream.
	httpResp, err := client.Call(context.Background(), prov.Endpoints.Chat, upReq.URL, "", upReq.Body, upReq.Headers)
	if err != nil {
		t.Fatalf("upstream call: %v", err)
	}

	// 13. Verify the upstream received the correct request body.
	var sentReq chat.ChatCompletionRequest
	if err := json.Unmarshal(capturedBody, &sentReq); err != nil {
		t.Fatalf("unmarshal captured body: %v", err)
	}
	if sentReq.Model != "test-model" {
		t.Errorf("mock upstream received model %q, expected test-model", sentReq.Model)
	}
	if len(sentReq.Messages) != 1 {
		t.Errorf("mock upstream received %d messages, expected 1", len(sentReq.Messages))
	}

	// 14. Translate response via ResToChat.TranslateResponse.
	resp, err := tr.TranslateResponse(context.Background(), httpResp, tReq, sess)
	if err != nil {
		t.Fatalf("TranslateResponse: %v", err)
	}

	// 15. Verify response is translated to correct Responses API format.
	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	var responsesResp responses.Response
	if err := json.Unmarshal(resp.Body, &responsesResp); err != nil {
		t.Fatalf("unmarshal translated response: %v", err)
	}
	if responsesResp.ID != "chatcmpl_integ_1" {
		t.Errorf("expected ID chatcmpl_integ_1, got %q", responsesResp.ID)
	}
	if responsesResp.Object != "response" {
		t.Errorf("expected object 'response', got %q", responsesResp.Object)
	}
	if len(responsesResp.Output) == 0 {
		t.Error("expected at least one output item")
	} else {
		out0 := responsesResp.Output[0]
		if out0.Type != "message" {
			t.Errorf("expected output[0].type 'message', got %q", out0.Type)
		}
		if out0.Role != "assistant" {
			t.Errorf("expected output[0].role 'assistant', got %q", out0.Role)
		}
		if len(out0.Content) == 0 || out0.Content[0].Text != "Hello from Chat integration test" {
			t.Errorf("unexpected output content: %+v", out0.Content)
		}
	}
	if responsesResp.Usage == nil {
		t.Error("expected usage in response")
	} else if responsesResp.Usage.TotalTokens != 12 {
		t.Errorf("expected total_tokens 12, got %d", responsesResp.Usage.TotalTokens)
	}

	// 16. Update session (reasoning records).
	tr.UpdateSession(sess, tReq, resp)

	// 17. Verify session state.
	if sess.ProviderID != "mock-prov" {
		t.Errorf("expected session provider mock-prov, got %q", sess.ProviderID)
	}
	if sess.ModelName != "test-model" {
		t.Errorf("expected session model test-model, got %q", sess.ModelName)
	}
	// ResToChat.TranslateResponse calls appendToSession, so messages should be appended.
	if len(sess.Messages) == 0 {
		t.Error("expected session messages to be appended after TranslateResponse")
	} else {
		lastMsg := sess.Messages[len(sess.Messages)-1]
		if lastMsg.Role != "assistant" {
			t.Errorf("expected last message role 'assistant', got %q", lastMsg.Role)
		}
		if lastMsg.Content != "Hello from Chat integration test" {
			t.Errorf("expected last message content %q, got %q", "Hello from Chat integration test", lastMsg.Content)
		}
	}
}

// buildResponsesRequest creates a JSON Responses API request body.
func buildResponsesRequest(t *testing.T, model, text string) []byte {
	t.Helper()
	textJSON, _ := json.Marshal(text)
	b, err := json.Marshal(responses.ResponseRequest{
		Model: model,
		Input: responses.ResponseInput{
			Items: []responses.ResponseInputItem{
				{Type: "message", Role: "user", Content: textJSON},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return b
}

func strPtr(s string) *string {
	return &s
}
