package chat

import (
	"encoding/json"
	"testing"
)

// TestChatCompletionRequestRoundtrip verifies request marshalling.
func TestChatCompletionRequestRoundtrip(t *testing.T) {
	reqJSON := `{
		"model": "gpt-4o-mini",
		"messages": [
			{
				"role": "user",
				"content": "hello"
			}
		],
		"temperature": 0.7,
		"max_completion_tokens": 1024,
		"stream": true,
		"stream_options": {
			"include_usage": true
		}
	}`

	var req ChatCompletionRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if req.Model != "gpt-4o-mini" {
		t.Errorf("expected model gpt-4o-mini, got %s", req.Model)
	}
	if req.Temperature == nil || *req.Temperature != 0.7 {
		t.Errorf("expected temperature 0.7, got %v", req.Temperature)
	}
	if req.MaxCompletionTokens == nil || *req.MaxCompletionTokens != 1024 {
		t.Errorf("expected max_completion_tokens 1024, got %v", req.MaxCompletionTokens)
	}
	if !req.Stream {
		t.Error("expected stream=true")
	}
	if req.StreamOptions == nil || req.StreamOptions.IncludeUsage == nil || !*req.StreamOptions.IncludeUsage {
		t.Error("expected stream_options.include_usage=true")
	}
	if len(req.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(req.Messages))
	}
	if req.Messages[0].Role != "user" {
		t.Errorf("expected role user, got %s", req.Messages[0].Role)
	}

	// Roundtrip.
	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	var req2 ChatCompletionRequest
	if err := json.Unmarshal(out, &req2); err != nil {
		t.Fatalf("unmarshal roundtrip: %v", err)
	}
	if req2.Model != "gpt-4o-mini" {
		t.Errorf("roundtrip model mismatch")
	}
	if req2.StreamOptions == nil || req2.StreamOptions.IncludeUsage == nil {
		t.Error("roundtrip stream_options lost")
	}
}

// TestChatCompletionRoundtrip verifies a full completion response with string content.
func TestChatCompletionRoundtrip(t *testing.T) {
	respJSON := `{
		"id": "chatcmpl-abc123",
		"object": "chat.completion",
		"created": 1721075653,
		"model": "gpt-4o-mini",
		"choices": [
			{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "Hello! How can I help you today?"
				},
				"finish_reason": "stop"
			}
		],
		"usage": {
			"prompt_tokens": 10,
			"completion_tokens": 20,
			"total_tokens": 30
		}
	}`

	var resp ChatCompletion
	if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.ID != "chatcmpl-abc123" {
		t.Errorf("expected id chatcmpl-abc123, got %s", resp.ID)
	}
	if resp.Object != "chat.completion" {
		t.Errorf("expected object chat.completion, got %s", resp.Object)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(resp.Choices))
	}
	if resp.Choices[0].FinishReason == nil || *resp.Choices[0].FinishReason != "stop" {
		t.Errorf("expected finish_reason stop, got %v", resp.Choices[0].FinishReason)
	}
	if resp.Usage == nil || resp.Usage.TotalTokens != 30 {
		t.Errorf("expected usage.total_tokens 30, got %v", resp.Usage)
	}

	// Verify string content survived.
	msg := resp.Choices[0].Message
	if msg.Content == nil || msg.Content.String == nil || *msg.Content.String != "Hello! How can I help you today?" {
		t.Errorf("expected content string, got %+v", msg.Content)
	}

	// Roundtrip.
	out, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	var resp2 ChatCompletion
	if err := json.Unmarshal(out, &resp2); err != nil {
		t.Fatalf("unmarshal roundtrip: %v", err)
	}
	if resp2.ID != "chatcmpl-abc123" {
		t.Errorf("roundtrip id mismatch")
	}
	if resp2.Usage == nil || resp2.Usage.TotalTokens != 30 {
		t.Errorf("roundtrip usage mismatch")
	}
}

// TestChatCompletionChunkRoundtrip verifies a streaming SSE chunk.
func TestChatCompletionChunkRoundtrip(t *testing.T) {
	chunkJSON := `{
		"id": "chatcmpl-abc123",
		"object": "chat.completion.chunk",
		"created": 1721075653,
		"model": "gpt-4o-mini",
		"choices": [
			{
				"index": 0,
				"delta": {
					"content": "Hello"
				},
				"finish_reason": null
			}
		]
	}`

	var chunk ChatCompletionChunk
	if err := json.Unmarshal([]byte(chunkJSON), &chunk); err != nil {
		t.Fatalf("unmarshal chunk: %v", err)
	}
	if chunk.Object != "chat.completion.chunk" {
		t.Errorf("expected object chat.completion.chunk, got %s", chunk.Object)
	}
	if len(chunk.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(chunk.Choices))
	}
	delta := chunk.Choices[0].Delta
	if delta.Content == nil || delta.Content.String == nil || *delta.Content.String != "Hello" {
		t.Errorf("expected delta content 'Hello', got %+v", delta.Content)
	}

	// Roundtrip.
	out, err := json.Marshal(chunk)
	if err != nil {
		t.Fatalf("marshal chunk: %v", err)
	}
	var chunk2 ChatCompletionChunk
	if err := json.Unmarshal(out, &chunk2); err != nil {
		t.Fatalf("unmarshal chunk roundtrip: %v", err)
	}
	if chunk2.Object != "chat.completion.chunk" {
		t.Errorf("roundtrip object mismatch")
	}
}

// TestToolCallRoundtrip verifies assistant messages with tool_calls (content=null).
func TestToolCallRoundtrip(t *testing.T) {
	respJSON := `{
		"id": "chatcmpl-tool123",
		"object": "chat.completion",
		"created": 1721075653,
		"model": "gpt-4o-mini",
		"choices": [
			{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": null,
					"tool_calls": [
						{
							"id": "call_abc123",
							"type": "function",
							"function": {
								"name": "get_weather",
								"arguments": "{\"location\":\"San Francisco\"}"
							}
						}
					]
				},
				"finish_reason": "tool_calls"
			}
		],
		"usage": {
			"prompt_tokens": 15,
			"completion_tokens": 25,
			"total_tokens": 40
		}
	}`

	var resp ChatCompletion
	if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
		t.Fatalf("unmarshal tool call response: %v", err)
	}
	msg := resp.Choices[0].Message
	if msg.Content != nil {
		t.Errorf("expected nil content for tool call message, got %+v", msg.Content)
	}
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(msg.ToolCalls))
	}
	tc := msg.ToolCalls[0]
	if tc.ID != "call_abc123" {
		t.Errorf("expected tool call id call_abc123, got %s", tc.ID)
	}
	if tc.Type != "function" {
		t.Errorf("expected tool call type function, got %s", tc.Type)
	}
	if tc.Function.Name != "get_weather" {
		t.Errorf("expected function name get_weather, got %s", tc.Function.Name)
	}
	if tc.Function.Arguments != `{"location":"San Francisco"}` {
		t.Errorf("expected function arguments, got %s", tc.Function.Arguments)
	}

	// Roundtrip.
	out, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal tool call response: %v", err)
	}
	var resp2 ChatCompletion
	if err := json.Unmarshal(out, &resp2); err != nil {
		t.Fatalf("unmarshal tool call roundtrip: %v", err)
	}
	msg2 := resp2.Choices[0].Message
	if msg2.Content != nil {
		t.Errorf("roundtrip: expected nil content")
	}
	if len(msg2.ToolCalls) != 1 {
		t.Fatalf("roundtrip: expected 1 tool call, got %d", len(msg2.ToolCalls))
	}
	if msg2.ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("roundtrip function name mismatch")
	}
}

// TestContentPartsRoundtrip verifies array-based content (multimodal).
func TestContentPartsRoundtrip(t *testing.T) {
	respJSON := `{
		"id": "chatcmpl-multi123",
		"object": "chat.completion",
		"created": 1721075653,
		"model": "gpt-4o",
		"choices": [
			{
				"index": 0,
				"message": {
					"role": "user",
					"content": [
						{
							"type": "text",
							"text": "What's in this image?"
						},
						{
							"type": "image_url",
							"image_url": {
								"url": "https://example.com/image.png",
								"detail": "high"
							}
						}
					]
				},
				"finish_reason": "stop"
			}
		],
		"usage": {
			"prompt_tokens": 100,
			"completion_tokens": 50,
			"total_tokens": 150
		}
	}`

	var resp ChatCompletion
	if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
		t.Fatalf("unmarshal content parts response: %v", err)
	}
	msg := resp.Choices[0].Message
	if msg.Content == nil || msg.Content.Parts == nil {
		t.Fatalf("expected content parts, got %+v", msg.Content)
	}
	if len(msg.Content.Parts) != 2 {
		t.Fatalf("expected 2 content parts, got %d", len(msg.Content.Parts))
	}
	p0 := msg.Content.Parts[0]
	if p0.Type != "text" || p0.Text != "What's in this image?" {
		t.Errorf("expected text part, got type=%s text=%s", p0.Type, p0.Text)
	}
	p1 := msg.Content.Parts[1]
	if p1.Type != "image_url" || p1.ImageURL == nil || p1.ImageURL.URL != "https://example.com/image.png" {
		t.Errorf("expected image_url part, got %+v", p1)
	}
	if p1.ImageURL.Detail != "high" {
		t.Errorf("expected detail high, got %s", p1.ImageURL.Detail)
	}

	// Roundtrip.
	out, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal content parts response: %v", err)
	}
	var resp2 ChatCompletion
	if err := json.Unmarshal(out, &resp2); err != nil {
		t.Fatalf("unmarshal content parts roundtrip: %v", err)
	}
	msg2 := resp2.Choices[0].Message
	if msg2.Content == nil || len(msg2.Content.Parts) != 2 {
		t.Errorf("roundtrip content parts count mismatch")
	}
}

// TestToolChoiceObjectRoundtrip verifies a request with a named tool choice.
func TestToolChoiceObjectRoundtrip(t *testing.T) {
	reqJSON := `{
		"model": "gpt-4o-mini",
		"messages": [
			{"role": "user", "content": "call get_weather"}
		],
		"tools": [
			{
				"type": "function",
				"function": {
					"name": "get_weather",
					"description": "Get the weather for a location",
					"parameters": {
						"type": "object",
						"properties": {
							"location": {"type": "string"}
						},
						"required": ["location"]
					}
				}
			}
		],
		"tool_choice": {
			"type": "function",
			"function": {
				"name": "get_weather"
			}
		}
	}`

	var req ChatCompletionRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		t.Fatalf("unmarshal tool choice request: %v", err)
	}
	if req.ToolChoice == nil || req.ToolChoice.Object == nil {
		t.Fatalf("expected named tool choice object, got %+v", req.ToolChoice)
	}
	if req.ToolChoice.Object.Function.Name != "get_weather" {
		t.Errorf("expected tool choice function name get_weather")
	}
	if len(req.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(req.Tools))
	}
	if req.Tools[0].Function.Parameters == nil {
		t.Error("expected tool function parameters")
	}

	// Roundtrip.
	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal tool choice request: %v", err)
	}
	var req2 ChatCompletionRequest
	if err := json.Unmarshal(out, &req2); err != nil {
		t.Fatalf("unmarshal tool choice roundtrip: %v", err)
	}
	if req2.ToolChoice == nil || req2.ToolChoice.Object == nil {
		t.Errorf("roundtrip tool choice lost")
	}
}

// TestStreamingDeltaToolCalls verifies incremental tool calls in streaming deltas.
func TestStreamingDeltaToolCalls(t *testing.T) {
	chunkJSON := `{
		"id": "chatcmpl-stream-tools",
		"object": "chat.completion.chunk",
		"created": 1721075653,
		"model": "gpt-4o-mini",
		"choices": [
			{
				"index": 0,
				"delta": {
					"tool_calls": [
						{
							"index": 0,
							"id": "call_abc123",
							"type": "function",
							"function": {
								"name": "get_weather",
								"arguments": "{\"location\":\"San Francisco\"}"
							}
						}
					]
				},
				"finish_reason": null
			}
		]
	}`

	var chunk ChatCompletionChunk
	if err := json.Unmarshal([]byte(chunkJSON), &chunk); err != nil {
		t.Fatalf("unmarshal streaming tool calls chunk: %v", err)
	}
	delta := chunk.Choices[0].Delta
	if len(delta.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call in delta, got %d", len(delta.ToolCalls))
	}
	tc := delta.ToolCalls[0]
	if tc.Index != 0 {
		t.Errorf("expected index 0, got %d", tc.Index)
	}
	if tc.ID != "call_abc123" {
		t.Errorf("expected id call_abc123, got %s", tc.ID)
	}
	if tc.Function == nil || tc.Function.Name != "get_weather" {
		t.Errorf("expected function name get_weather")
	}

	// Roundtrip.
	out, err := json.Marshal(chunk)
	if err != nil {
		t.Fatalf("marshal streaming tool calls chunk: %v", err)
	}
	var chunk2 ChatCompletionChunk
	if err := json.Unmarshal(out, &chunk2); err != nil {
		t.Fatalf("unmarshal streaming tool calls roundtrip: %v", err)
	}
	delta2 := chunk2.Choices[0].Delta
	if len(delta2.ToolCalls) != 1 {
		t.Errorf("roundtrip tool calls count mismatch")
	}
}
