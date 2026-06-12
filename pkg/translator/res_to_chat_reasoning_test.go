package translator

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/tianyuansun/ai-gateway/pkg/session"
)

func TestResToChat_UpdateSessionNoEmptyReasoning(t *testing.T) {
	tr := &ResToChat{}
	s := &session.Session{ID: "sess-1"}

	// Non-reasoning model response.
	rawJSON := `{
		"id": "chatcmpl-1",
		"object": "chat.completion",
		"choices": [{
			"index": 0,
			"message": {
				"role": "assistant",
				"content": "simple answer"
			}
		}]
	}`
	httpResp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader([]byte(rawJSON))),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}

	req := &Request{Model: "gpt-4o", APIFormat: FormatResponses}
	resp, err := tr.TranslateResponse(context.Background(), httpResp, req, s)
	if err != nil {
		t.Fatalf("TranslateResponse: %v", err)
	}

	tr.UpdateSession(s, req, resp)

	if len(s.ReasoningRecords) != 0 {
		t.Errorf("expected 0 reasoning records for non-reasoning response, got %d", len(s.ReasoningRecords))
	}
}

func TestResToChat_UpdateSessionStoresReasoning(t *testing.T) {
	tr := &ResToChat{}
	s := &session.Session{
		ID: "sess-1",
		Messages: []session.Message{
			{Role: "user", Content: "think step by step"},
		},
	}

	// Simulate a Chat Completions response with reasoning_content.
	chatResp := ChatResponse{
		ID:     "chatcmpl-1",
		Object: "chat.completion",
		Choices: []ChatChoice{{
			Index: 0,
			Message: ChatMessage{
				Role:    "assistant",
				Content: "The answer is 4.",
				// Reasoning content comes as an additional field.
			},
		}},
	}

	// Inject reasoning_content via a raw JSON response (OpenAI style).
	rawJSON := `{
		"id": "chatcmpl-1",
		"object": "chat.completion",
		"choices": [{
			"index": 0,
			"message": {
				"role": "assistant",
				"content": "The answer is 4.",
				"reasoning_content": "Let me count... 2+2=4"
			}
		}]
	}`
	httpResp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader([]byte(rawJSON))),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}

	// Call TranslateResponse to populate the response and session.
	req := &Request{Model: "deepseek-r1", APIFormat: FormatResponses}
	resp, err := tr.TranslateResponse(context.Background(), httpResp, req, s)
	if err != nil {
		t.Fatalf("TranslateResponse: %v", err)
	}

	// Now call UpdateSession — it should extract reasoning.
	tr.UpdateSession(s, req, resp)

	if len(s.ReasoningRecords) == 0 {
		t.Fatal("expected reasoning records to be populated")
	}
	if s.ReasoningRecords[0].Content != "Let me count... 2+2=4" {
		t.Errorf("expected reasoning 'Let me count... 2+2=4', got %q", s.ReasoningRecords[0].Content)
	}

	// Verify chatResp still got the message content right.
	_ = chatResp // silence unused warning
}

func TestResToChat_TranslateRequestInjectsReasoning(t *testing.T) {
	tr := &ResToChat{}
	s := &session.Session{
		ID: "sess-1",
		Messages: []session.Message{
			{Role: "user", Content: "think step by step"},
			{Role: "assistant", Content: "The answer is 4."},
			{Role: "user", Content: "continue"},
		},
		ReasoningRecords: []session.Reasoning{
			{Content: "Previous step-by-step reasoning..."},
		},
	}

	body := responsesRequest(t, "deepseek-r1", "continue")
	req := &Request{
		Model:     "deepseek-r1",
		APIFormat: FormatResponses,
		Body:      body,
	}

	upReq, err := tr.TranslateRequest(context.Background(), req, s)
	if err != nil {
		t.Fatalf("TranslateRequest: %v", err)
	}

	var chatReq ChatRequest
	if err := json.Unmarshal(upReq.Body, &chatReq); err != nil {
		t.Fatalf("unmarshal upstream body: %v", err)
	}

	// Reasoning should be injected into the assistant message.
	foundReasoning := false
	for _, msg := range chatReq.Messages {
		// Check if reasoning_content was injected
		var raw map[string]any
		rawBytes, _ := json.Marshal(msg)
		json.Unmarshal(rawBytes, &raw)
		if rc, ok := raw["reasoning_content"]; ok && rc != nil {
			foundReasoning = true
			break
		}
	}
	if !foundReasoning {
		t.Error("expected reasoning_content to be injected into upstream messages")
	}
}
