package anthropic

import (
	"encoding/json"
	"testing"
)

func TestMessageRequestRoundtrip(t *testing.T) {
	// A typical Messages API request.
	reqJSON := `{
		"model": "claude-sonnet-4-6",
		"max_tokens": 1024,
		"stream": true,
		"messages": [
			{
				"role": "user",
				"content": [
					{"type": "text", "text": "hello"}
				]
			}
		]
	}`

	var req MessageRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if req.Model != "claude-sonnet-4-6" {
		t.Errorf("expected model claude-sonnet-4-6, got %s", req.Model)
	}
	if req.MaxTokens != 1024 {
		t.Errorf("expected max_tokens 1024, got %d", req.MaxTokens)
	}
	if !req.Stream {
		t.Error("expected stream=true")
	}
	if len(req.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(req.Messages))
	}

	// Roundtrip: marshal back.
	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	var req2 MessageRequest
	if err := json.Unmarshal(out, &req2); err != nil {
		t.Fatalf("unmarshal roundtrip: %v", err)
	}
	if req2.Model != "claude-sonnet-4-6" {
		t.Errorf("roundtrip model mismatch")
	}
}

func TestMessageResponseRoundtrip(t *testing.T) {
	// Anthropic Messages API response.
	respJSON := `{
		"id": "msg_abc123",
		"type": "message",
		"role": "assistant",
		"model": "claude-sonnet-4-6",
		"content": [
			{"type": "text", "text": "Hello! How can I help?"}
		],
		"stop_reason": "end_turn",
		"usage": {
			"input_tokens": 10,
			"output_tokens": 20
		}
	}`

	var resp MessageResponse
	if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.ID != "msg_abc123" {
		t.Errorf("expected id msg_abc123, got %s", resp.ID)
	}
	if len(resp.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(resp.Content))
	}

	// Roundtrip.
	out, _ := json.Marshal(resp)
	var resp2 MessageResponse
	if err := json.Unmarshal(out, &resp2); err != nil {
		t.Fatalf("unmarshal roundtrip: %v", err)
	}
	if resp2.ID != "msg_abc123" {
		t.Errorf("roundtrip id mismatch")
	}
	if resp2.Usage.OutputTokens != 20 {
		t.Errorf("roundtrip usage mismatch")
	}
}

func TestThinkingContentBlock(t *testing.T) {
	// Responses can include thinking/reasoning content blocks.
	respJSON := `{
		"id": "msg_think",
		"type": "message",
		"role": "assistant",
		"model": "claude-opus-4-6",
		"content": [
			{"type": "thinking", "thinking": "Let me reason...", "signature": "sig1"},
			{"type": "text", "text": "The answer is 42."}
		],
		"stop_reason": "end_turn",
		"usage": {"input_tokens": 15, "output_tokens": 30}
	}`

	var resp MessageResponse
	if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
		t.Fatalf("unmarshal thinking response: %v", err)
	}
	if len(resp.Content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(resp.Content))
	}
	if resp.Content[0].Type != "thinking" {
		t.Errorf("expected first block type=thinking, got %s", resp.Content[0].Type)
	}
}
