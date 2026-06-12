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

// -- Deep roundtrip tests for Issue #52 -------------------------------------------

// TestToolUseContentBlockRoundtrip verifies tool_use content blocks with JSON input
// survive a marshal-unmarshal roundtrip.
func TestToolUseContentBlockRoundtrip(t *testing.T) {
	respJSON := `{
		"id": "msg_tooluse",
		"type": "message",
		"role": "assistant",
		"model": "claude-sonnet-4-6",
		"content": [
			{
				"type": "tool_use",
				"id": "toolu_01ABC123",
				"name": "get_weather",
				"input": {"location": "San Francisco", "unit": "celsius"}
			}
		],
		"stop_reason": "tool_use",
		"usage": {"input_tokens": 50, "output_tokens": 30}
	}`

	var resp MessageResponse
	if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
		t.Fatalf("unmarshal tool_use response: %v", err)
	}
	if len(resp.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(resp.Content))
	}
	cb := resp.Content[0]
	if cb.Type != "tool_use" {
		t.Errorf("expected type=tool_use, got %s", cb.Type)
	}
	if cb.ID != "toolu_01ABC123" {
		t.Errorf("expected id=toolu_01ABC123, got %s", cb.ID)
	}
	if cb.Name != "get_weather" {
		t.Errorf("expected name=get_weather, got %s", cb.Name)
	}
	if cb.Input == nil {
		t.Fatal("expected non-nil input")
	}

	// Roundtrip.
	out, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal tool_use response: %v", err)
	}
	var resp2 MessageResponse
	if err := json.Unmarshal(out, &resp2); err != nil {
		t.Fatalf("unmarshal tool_use roundtrip: %v", err)
	}
	cb2 := resp2.Content[0]
	if cb2.Type != "tool_use" {
		t.Errorf("roundtrip: expected type=tool_use, got %s", cb2.Type)
	}
	if cb2.ID != "toolu_01ABC123" {
		t.Errorf("roundtrip: id mismatch")
	}
	if cb2.Name != "get_weather" {
		t.Errorf("roundtrip: name mismatch")
	}
	if cb2.Input == nil {
		t.Fatal("roundtrip: expected non-nil input")
	}
	var inputMap map[string]interface{}
	if err := json.Unmarshal(cb2.Input, &inputMap); err != nil {
		t.Fatalf("roundtrip: failed to unmarshal input: %v", err)
	}
	if inputMap["location"] != "San Francisco" {
		t.Errorf("roundtrip: input location mismatch")
	}
	if inputMap["unit"] != "celsius" {
		t.Errorf("roundtrip: input unit mismatch")
	}
}

// TestToolResultContentBlockRoundtrip verifies tool_result blocks in a request
// roundtrip correctly.
func TestToolResultContentBlockRoundtrip(t *testing.T) {
	reqJSON := `{
		"model": "claude-sonnet-4-6",
		"max_tokens": 1024,
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "What is the weather?"}]},
			{
				"role": "assistant",
				"content": [
					{
						"type": "tool_use",
						"id": "toolu_01A",
						"name": "get_weather",
						"input": {"location": "SF"}
					}
				]
			},
			{
				"role": "user",
				"content": [
					{
						"type": "tool_result",
						"tool_use_id": "toolu_01A",
						"content": "Sunny, 22C",
						"is_error": false
					}
				]
			}
		]
	}`

	var req MessageRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		t.Fatalf("unmarshal tool_result request: %v", err)
	}
	if len(req.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(req.Messages))
	}
	msg := req.Messages[2]
	if msg.Role != "user" {
		t.Errorf("expected role=user for tool_result, got %s", msg.Role)
	}
	if len(msg.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(msg.Content))
	}
	cb := msg.Content[0]
	if cb.Type != "tool_result" {
		t.Errorf("expected type=tool_result, got %s", cb.Type)
	}
	if cb.ToolUseID != "toolu_01A" {
		t.Errorf("expected tool_use_id=toolu_01A, got %s", cb.ToolUseID)
	}
	if cb.Content != "Sunny, 22C" {
		t.Errorf("expected content, got %s", cb.Content)
	}

	// Roundtrip.
	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal tool_result request: %v", err)
	}
	var req2 MessageRequest
	if err := json.Unmarshal(out, &req2); err != nil {
		t.Fatalf("unmarshal tool_result roundtrip: %v", err)
	}
	if len(req2.Messages) != 3 {
		t.Fatalf("roundtrip: expected 3 messages, got %d", len(req2.Messages))
	}
	cb2 := req2.Messages[2].Content[0]
	if cb2.Type != "tool_result" {
		t.Errorf("roundtrip: type mismatch")
	}
	if cb2.ToolUseID != "toolu_01A" {
		t.Errorf("roundtrip: tool_use_id mismatch")
	}
	if cb2.Content != "Sunny, 22C" {
		t.Errorf("roundtrip: content mismatch")
	}
}

// TestThinkingBlockDeepRoundtrip verifies thinking blocks with signature survive
// a full marshal-unmarshal cycle.
func TestThinkingBlockDeepRoundtrip(t *testing.T) {
	respJSON := `{
		"id": "msg_think_deep",
		"type": "message",
		"role": "assistant",
		"model": "claude-opus-4-6",
		"content": [
			{
				"type": "thinking",
				"thinking": "I need to analyze this carefully. The user asked about...",
				"signature": "EqoBCkgIARJPCkdaR...truncated"
			},
			{"type": "text", "text": "Based on my analysis, here is the answer."}
		],
		"stop_reason": "end_turn",
		"usage": {"input_tokens": 200, "output_tokens": 500}
	}`

	var resp MessageResponse
	if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
		t.Fatalf("unmarshal thinking deep response: %v", err)
	}
	if len(resp.Content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(resp.Content))
	}
	thinkBlock := resp.Content[0]
	if thinkBlock.Type != "thinking" {
		t.Errorf("expected type=thinking, got %s", thinkBlock.Type)
	}
	if thinkBlock.Thinking != "I need to analyze this carefully. The user asked about..." {
		t.Errorf("thinking text mismatch")
	}
	if thinkBlock.Signature != "EqoBCkgIARJPCkdaR...truncated" {
		t.Errorf("signature mismatch")
	}

	// Roundtrip.
	out, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal thinking response: %v", err)
	}
	var resp2 MessageResponse
	if err := json.Unmarshal(out, &resp2); err != nil {
		t.Fatalf("unmarshal thinking roundtrip: %v", err)
	}
	cb := resp2.Content[0]
	if cb.Type != "thinking" {
		t.Errorf("roundtrip: expected type=thinking, got %s", cb.Type)
	}
	if cb.Thinking != "I need to analyze this carefully. The user asked about..." {
		t.Errorf("roundtrip: thinking text mismatch")
	}
	if cb.Signature != "EqoBCkgIARJPCkdaR...truncated" {
		t.Errorf("roundtrip: signature mismatch")
	}
}

// TestNullOmittedFieldsRoundtrip verifies that optional fields which are omitted
// or explicitly null remain zero-valued / nil through a roundtrip.
func TestNullOmittedFieldsRoundtrip(t *testing.T) {
	reqJSON := `{
		"model": "claude-sonnet-4-6",
		"max_tokens": 500,
		"stream": false,
		"top_k": null,
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "hi"}]}
		]
	}`

	var req MessageRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		t.Fatalf("unmarshal request with null/omitted fields: %v", err)
	}
	if req.Temperature != nil {
		t.Errorf("expected omitted temperature to be nil, got %v", *req.Temperature)
	}
	if req.TopP != nil {
		t.Errorf("expected omitted top_p to be nil, got %v", *req.TopP)
	}
	if req.TopK != nil {
		t.Errorf("expected null top_k to be nil, got %v", *req.TopK)
	}
	if req.StopSequences != nil {
		t.Errorf("expected omitted stop_sequences to be nil, got %v", req.StopSequences)
	}

	// Roundtrip.
	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal null/omitted request: %v", err)
	}
	var req2 MessageRequest
	if err := json.Unmarshal(out, &req2); err != nil {
		t.Fatalf("unmarshal null/omitted roundtrip: %v", err)
	}
	if req2.Temperature != nil {
		t.Errorf("roundtrip: expected temperature nil, got %v", *req2.Temperature)
	}
	if req2.TopP != nil {
		t.Errorf("roundtrip: expected top_p nil, got %v", *req2.TopP)
	}
	if req2.TopK != nil {
		t.Errorf("roundtrip: expected top_k nil, got %v", *req2.TopK)
	}
}

// TestMultiMessageArrayRoundtrip verifies multi-turn conversation arrays
// (user + assistant + user) survive a full roundtrip.
func TestMultiMessageArrayRoundtrip(t *testing.T) {
	reqJSON := `{
		"model": "claude-sonnet-4-6",
		"max_tokens": 1024,
		"messages": [
			{
				"role": "user",
				"content": [{"type": "text", "text": "What's the capital of France?"}]
			},
			{
				"role": "assistant",
				"content": [{"type": "text", "text": "The capital of France is Paris."}]
			},
			{
				"role": "user",
				"content": [{"type": "text", "text": "What about Germany?"}]
			}
		]
	}`

	var req MessageRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		t.Fatalf("unmarshal multi-message request: %v", err)
	}
	if len(req.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(req.Messages))
	}
	if req.Messages[0].Role != "user" {
		t.Errorf("msg[0] role mismatch: %s", req.Messages[0].Role)
	}
	if req.Messages[1].Role != "assistant" {
		t.Errorf("msg[1] role mismatch: %s", req.Messages[1].Role)
	}
	if req.Messages[2].Role != "user" {
		t.Errorf("msg[2] role mismatch: %s", req.Messages[2].Role)
	}
	// Also check text content survived.
	if req.Messages[0].Content[0].Text != "What's the capital of France?" {
		t.Errorf("msg[0] text mismatch")
	}
	if req.Messages[1].Content[0].Text != "The capital of France is Paris." {
		t.Errorf("msg[1] text mismatch")
	}

	// Roundtrip.
	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal multi-message request: %v", err)
	}
	var req2 MessageRequest
	if err := json.Unmarshal(out, &req2); err != nil {
		t.Fatalf("unmarshal multi-message roundtrip: %v", err)
	}
	if len(req2.Messages) != 3 {
		t.Fatalf("roundtrip: expected 3 messages, got %d", len(req2.Messages))
	}
	if req2.Messages[0].Role != "user" {
		t.Errorf("roundtrip: msg[0] role mismatch")
	}
	if req2.Messages[1].Role != "assistant" {
		t.Errorf("roundtrip: msg[1] role mismatch")
	}
	if req2.Messages[2].Role != "user" {
		t.Errorf("roundtrip: msg[2] role mismatch")
	}
}

// TestSystemContentStringVsArrayRoundtrip verifies that system content survives
// a roundtrip both as a plain string and as an array of text blocks.
func TestSystemContentStringVsArrayRoundtrip(t *testing.T) {
	// Test 1: system as a plain string.
	reqStrJSON := `{
		"model": "claude-sonnet-4-6",
		"max_tokens": 1024,
		"system": "You are a helpful assistant.",
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "hello"}]}
		]
	}`

	var reqStr MessageRequest
	if err := json.Unmarshal([]byte(reqStrJSON), &reqStr); err != nil {
		t.Fatalf("unmarshal system string request: %v", err)
	}
	if reqStr.System == nil {
		t.Fatal("expected non-nil system")
	}
	if reqStr.System.String == nil {
		t.Fatal("expected system string, got blocks")
	}
	if *reqStr.System.String != "You are a helpful assistant." {
		t.Errorf("system string mismatch: %s", *reqStr.System.String)
	}

	outStr, err := json.Marshal(reqStr)
	if err != nil {
		t.Fatalf("marshal system string: %v", err)
	}
	var reqStr2 MessageRequest
	if err := json.Unmarshal(outStr, &reqStr2); err != nil {
		t.Fatalf("unmarshal system string roundtrip: %v", err)
	}
	if reqStr2.System == nil || reqStr2.System.String == nil {
		t.Fatal("roundtrip: expected system string")
	}
	if *reqStr2.System.String != "You are a helpful assistant." {
		t.Errorf("roundtrip: system string mismatch")
	}

	// Test 2: system as an array of text blocks.
	reqArrJSON := `{
		"model": "claude-sonnet-4-6",
		"max_tokens": 1024,
		"system": [
			{"type": "text", "text": "You are a helpful assistant."},
			{"type": "text", "text": "You are also extremely concise.", "cache_control": {"type": "ephemeral"}}
		],
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "hello"}]}
		]
	}`

	var reqArr MessageRequest
	if err := json.Unmarshal([]byte(reqArrJSON), &reqArr); err != nil {
		t.Fatalf("unmarshal system array request: %v", err)
	}
	if reqArr.System == nil {
		t.Fatal("expected non-nil system")
	}
	if reqArr.System.String != nil {
		t.Fatal("expected system blocks, got string")
	}
	if len(reqArr.System.Blocks) != 2 {
		t.Fatalf("expected 2 system blocks, got %d", len(reqArr.System.Blocks))
	}
	if reqArr.System.Blocks[0].Text != "You are a helpful assistant." {
		t.Errorf("system block[0] text mismatch")
	}
	if reqArr.System.Blocks[1].Text != "You are also extremely concise." {
		t.Errorf("system block[1] text mismatch")
	}
	if reqArr.System.Blocks[1].CacheControl == nil || reqArr.System.Blocks[1].CacheControl.Type != "ephemeral" {
		t.Errorf("system block[1] cache_control mismatch")
	}

	outArr, err := json.Marshal(reqArr)
	if err != nil {
		t.Fatalf("marshal system array: %v", err)
	}
	var reqArr2 MessageRequest
	if err := json.Unmarshal(outArr, &reqArr2); err != nil {
		t.Fatalf("unmarshal system array roundtrip: %v", err)
	}
	if reqArr2.System == nil || reqArr2.System.String != nil {
		t.Fatal("roundtrip: expected system blocks")
	}
	if len(reqArr2.System.Blocks) != 2 {
		t.Fatalf("roundtrip: expected 2 system blocks, got %d", len(reqArr2.System.Blocks))
	}
	if reqArr2.System.Blocks[0].Text != "You are a helpful assistant." {
		t.Errorf("roundtrip: system block[0] text mismatch")
	}
}
