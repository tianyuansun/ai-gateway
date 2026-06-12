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

func TestResToAnth_UpdateSessionStoresThinking(t *testing.T) {
	tr := &ResToAnth{}
	s := &session.Session{
		ID: "sess-1",
		Messages: []session.Message{
			{Role: "user", Content: "think step by step"},
		},
	}

	// Anthropic response with thinking content blocks.
	rawJSON := `{
		"id": "msg_1",
		"type": "message",
		"role": "assistant",
		"content": [
			{"type": "thinking", "thinking": "Let me reason about this...", "signature": "sig1"},
			{"type": "text", "text": "final answer"}
		],
		"usage": {"input_tokens": 10, "output_tokens": 30}
	}`
	httpResp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader([]byte(rawJSON))),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}

	req := &Request{Model: "claude-sonnet-4-6", APIFormat: FormatResponses}
	resp, err := tr.TranslateResponse(context.Background(), httpResp, req, s)
	if err != nil {
		t.Fatalf("TranslateResponse: %v", err)
	}

	tr.UpdateSession(s, req, resp)

	if len(s.ReasoningRecords) == 0 {
		t.Fatal("expected reasoning records to be populated")
	}
	if s.ReasoningRecords[0].Content != "Let me reason about this..." {
		t.Errorf("expected thinking content, got %q", s.ReasoningRecords[0].Content)
	}
}

func TestResToAnth_TranslateRequestInjectsThinking(t *testing.T) {
	tr := &ResToAnth{}
	s := &session.Session{
		ID: "sess-1",
		Messages: []session.Message{
			{Role: "user", Content: "think step by step"},
			{Role: "assistant", Content: "final answer"},
			{Role: "user", Content: "explain more"},
		},
		ReasoningRecords: []session.Reasoning{
			{Content: "Previous thinking process..."},
		},
	}

	body := responsesRequest(t, "claude-sonnet-4-6", "explain more")
	req := &Request{
		Model:     "claude-sonnet-4-6",
		APIFormat: FormatResponses,
		Body:      body,
	}

	upReq, err := tr.TranslateRequest(context.Background(), req, s)
	if err != nil {
		t.Fatalf("TranslateRequest: %v", err)
	}

	var anthReq AnthropicRequest
	if err := json.Unmarshal(upReq.Body, &anthReq); err != nil {
		t.Fatalf("unmarshal upstream body: %v", err)
	}

	// Find the assistant message with thinking injected.
	foundThinking := false
	for _, msg := range anthReq.Messages {
		if msg.Role == "assistant" {
			for _, c := range msg.Content {
				if c.Type == "thinking" && c.Thinking != "" {
					foundThinking = true
				}
			}
		}
	}
	if !foundThinking {
		t.Error("expected thinking block to be injected into assistant message")
	}
}
