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

// -- Deep roundtrip tests for Issue #52 -------------------------------------------

// TestToolCallsDeepRoundtrip verifies tool calls with complex nested JSON arguments
// survive a full roundtrip.
func TestToolCallsDeepRoundtrip(t *testing.T) {
	respJSON := `{
		"id": "chatcmpl-tool-deep",
		"object": "chat.completion",
		"created": 1721075653,
		"model": "gpt-4o",
		"choices": [
			{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": null,
					"tool_calls": [
						{
							"id": "call_001",
							"type": "function",
							"function": {
								"name": "search",
								"arguments": "{\"query\":\"AI safety\",\"filters\":{\"year\":2025,\"type\":\"paper\"}}"
							}
						},
						{
							"id": "call_002",
							"type": "function",
							"function": {
								"name": "get_data",
								"arguments": "{\"source\":\"db\",\"limit\":50}"
							}
						}
					]
				},
				"finish_reason": "tool_calls"
			}
		],
		"usage": {
			"prompt_tokens": 30,
			"completion_tokens": 60,
			"total_tokens": 90
		}
	}`

	var resp ChatCompletion
	if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
		t.Fatalf("unmarshal tool calls deep response: %v", err)
	}
	msg := resp.Choices[0].Message
	if msg.Content != nil {
		t.Errorf("expected nil content for tool call message")
	}
	if len(msg.ToolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].Function.Name != "search" {
		t.Errorf("tool_calls[0] name mismatch")
	}
	if msg.ToolCalls[1].Function.Name != "get_data" {
		t.Errorf("tool_calls[1] name mismatch")
	}
	// Verify complex arguments.
	if msg.ToolCalls[0].Function.Arguments != `{"query":"AI safety","filters":{"year":2025,"type":"paper"}}` {
		t.Errorf("tool_calls[0] arguments mismatch")
	}

	// Roundtrip.
	out, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal tool calls deep response: %v", err)
	}
	var resp2 ChatCompletion
	if err := json.Unmarshal(out, &resp2); err != nil {
		t.Fatalf("unmarshal tool calls deep roundtrip: %v", err)
	}
	msg2 := resp2.Choices[0].Message
	if len(msg2.ToolCalls) != 2 {
		t.Fatalf("roundtrip: expected 2 tool calls, got %d", len(msg2.ToolCalls))
	}
	if msg2.ToolCalls[0].Function.Name != "search" {
		t.Errorf("roundtrip: tool_calls[0] name mismatch")
	}
	if msg2.ToolCalls[1].Function.Name != "get_data" {
		t.Errorf("roundtrip: tool_calls[1] name mismatch")
	}
}

// TestImageContentDeepRoundtrip verifies an image content part with all fields
// survives a roundtrip.
func TestImageContentDeepRoundtrip(t *testing.T) {
	reqJSON := `{
		"model": "gpt-4o",
		"messages": [
			{
				"role": "user",
				"content": [
					{
						"type": "text",
						"text": "Describe this image in detail."
					},
					{
						"type": "image_url",
						"image_url": {
							"url": "https://example.com/photo.jpg",
							"detail": "low"
						}
					}
				]
			}
		]
	}`

	var req ChatCompletionRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		t.Fatalf("unmarshal image request: %v", err)
	}
	msg := req.Messages[0]
	if msg.Content == nil || msg.Content.Parts == nil {
		t.Fatal("expected content parts")
	}
	if len(msg.Content.Parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(msg.Content.Parts))
	}
	imgPart := msg.Content.Parts[1]
	if imgPart.Type != "image_url" {
		t.Errorf("expected type=image_url, got %s", imgPart.Type)
	}
	if imgPart.ImageURL == nil {
		t.Fatal("expected non-nil image_url")
	}
	if imgPart.ImageURL.URL != "https://example.com/photo.jpg" {
		t.Errorf("image url mismatch")
	}
	if imgPart.ImageURL.Detail != "low" {
		t.Errorf("image detail mismatch")
	}

	// Roundtrip.
	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal image request: %v", err)
	}
	var req2 ChatCompletionRequest
	if err := json.Unmarshal(out, &req2); err != nil {
		t.Fatalf("unmarshal image roundtrip: %v", err)
	}
	parts2 := req2.Messages[0].Content.Parts
	if len(parts2) != 2 {
		t.Fatalf("roundtrip: expected 2 parts, got %d", len(parts2))
	}
	if parts2[1].ImageURL == nil || parts2[1].ImageURL.URL != "https://example.com/photo.jpg" {
		t.Errorf("roundtrip: image url mismatch")
	}
}

// TestStreamingDeltaDeepRoundtrip verifies a streaming chunk delta with role,
// refusal, and function_call survives a roundtrip.
func TestStreamingDeltaDeepRoundtrip(t *testing.T) {
	chunkJSON := `{
		"id": "chatcmpl-stream-deep",
		"object": "chat.completion.chunk",
		"created": 1721075653,
		"model": "gpt-4o-mini",
		"choices": [
			{
				"index": 0,
				"delta": {
					"role": "assistant",
					"content": "I'll help with",
					"refusal": null,
					"function_call": {
						"name": "search",
						"arguments": "{\"q\":\"test\"}"
					}
				},
				"finish_reason": null
			}
		],
		"usage": {
			"prompt_tokens": 5,
			"completion_tokens": 3,
			"total_tokens": 8
		}
	}`

	var chunk ChatCompletionChunk
	if err := json.Unmarshal([]byte(chunkJSON), &chunk); err != nil {
		t.Fatalf("unmarshal streaming deep chunk: %v", err)
	}
	delta := chunk.Choices[0].Delta
	if delta.Role == nil || *delta.Role != "assistant" {
		t.Errorf("expected delta role 'assistant', got %v", delta.Role)
	}
	if delta.Content == nil || delta.Content.String == nil || *delta.Content.String != "I'll help with" {
		t.Errorf("expected delta content, got %+v", delta.Content)
	}
	if delta.Refusal != nil {
		t.Errorf("expected nil refusal for null field")
	}
	if delta.FunctionCall == nil || delta.FunctionCall.Name != "search" {
		t.Errorf("expected function_call name 'search', got %v", delta.FunctionCall)
	}
	if chunk.Usage == nil || chunk.Usage.TotalTokens != 8 {
		t.Errorf("expected usage.total_tokens 8, got %v", chunk.Usage)
	}

	// Roundtrip.
	out, err := json.Marshal(chunk)
	if err != nil {
		t.Fatalf("marshal streaming deep chunk: %v", err)
	}
	var chunk2 ChatCompletionChunk
	if err := json.Unmarshal(out, &chunk2); err != nil {
		t.Fatalf("unmarshal streaming deep roundtrip: %v", err)
	}
	delta2 := chunk2.Choices[0].Delta
	if delta2.Content == nil || *delta2.Content.String != "I'll help with" {
		t.Errorf("roundtrip: content mismatch")
	}
	if delta2.FunctionCall == nil || delta2.FunctionCall.Name != "search" {
		t.Errorf("roundtrip: function_call mismatch")
	}
}

// TestLogprobsRoundtrip verifies logprobs data on a choice survives a roundtrip.
func TestLogprobsRoundtrip(t *testing.T) {
	respJSON := `{
		"id": "chatcmpl-logprobs",
		"object": "chat.completion",
		"created": 1721075653,
		"model": "gpt-4o",
		"choices": [
			{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "Hello"
				},
				"finish_reason": "stop",
				"logprobs": {
					"content": [
						{
							"token": "Hello",
							"logprob": -1.5,
							"bytes": [72, 101, 108, 108, 111],
							"top_logprobs": [
								{"token": "Hi", "logprob": -2.1, "bytes": [72, 105]}
							]
						}
					]
				}
			}
		],
		"usage": {
			"prompt_tokens": 10,
			"completion_tokens": 5,
			"total_tokens": 15
		}
	}`

	var resp ChatCompletion
	if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
		t.Fatalf("unmarshal logprobs response: %v", err)
	}
	lp := resp.Choices[0].Logprobs
	if lp == nil {
		t.Fatal("expected non-nil logprobs")
	}
	if len(lp.Content) != 1 {
		t.Fatalf("expected 1 logprob entry, got %d", len(lp.Content))
	}
	le := lp.Content[0]
	if le.Token != "Hello" {
		t.Errorf("expected token 'Hello', got %s", le.Token)
	}
	if le.Logprob != -1.5 {
		t.Errorf("expected logprob -1.5, got %f", le.Logprob)
	}
	if len(le.Bytes) != 5 {
		t.Errorf("expected 5 bytes, got %d", len(le.Bytes))
	}
	if len(le.TopLogprobs) != 1 {
		t.Errorf("expected 1 top_logprob entry, got %d", len(le.TopLogprobs))
	}

	// Roundtrip.
	out, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal logprobs response: %v", err)
	}
	var resp2 ChatCompletion
	if err := json.Unmarshal(out, &resp2); err != nil {
		t.Fatalf("unmarshal logprobs roundtrip: %v", err)
	}
	lp2 := resp2.Choices[0].Logprobs
	if lp2 == nil || len(lp2.Content) != 1 {
		t.Fatal("roundtrip: logprobs lost or count mismatch")
	}
	if lp2.Content[0].Token != "Hello" {
		t.Errorf("roundtrip: token mismatch")
	}
	if lp2.Content[0].Logprob != -1.5 {
		t.Errorf("roundtrip: logprob mismatch")
	}
}

// TestNullContentVsEmptyContent verifies that null content, empty string content,
// and absent content are distinguished through a roundtrip.
func TestNullContentVsEmptyContent(t *testing.T) {
	// Case 1: content is null (tool call message).
	nullJSON := `{
		"id": "chatcmpl-null",
		"object": "chat.completion",
		"created": 1721075653,
		"model": "gpt-4o",
		"choices": [
			{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": null,
					"tool_calls": [
						{
							"id": "call_x",
							"type": "function",
							"function": {"name": "f", "arguments": "{}"}
						}
					]
				},
				"finish_reason": "tool_calls"
			}
		],
		"usage": {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}
	}`

	var nullResp ChatCompletion
	if err := json.Unmarshal([]byte(nullJSON), &nullResp); err != nil {
		t.Fatalf("unmarshal null content: %v", err)
	}
	if nullResp.Choices[0].Message.Content != nil {
		t.Errorf("expected nil Content for null content JSON")
	}
	outNull, _ := json.Marshal(nullResp)
	var nullResp2 ChatCompletion
	json.Unmarshal(outNull, &nullResp2)
	if nullResp2.Choices[0].Message.Content != nil {
		t.Errorf("roundtrip: expected nil Content for null")
	}

	// Case 2: content is an empty string.
	emptyJSON := `{
		"id": "chatcmpl-empty",
		"object": "chat.completion",
		"created": 1721075653,
		"model": "gpt-4o",
		"choices": [
			{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": ""
				},
				"finish_reason": "stop"
			}
		],
		"usage": {"prompt_tokens": 1, "completion_tokens": 0, "total_tokens": 1}
	}`

	var emptyResp ChatCompletion
	if err := json.Unmarshal([]byte(emptyJSON), &emptyResp); err != nil {
		t.Fatalf("unmarshal empty content: %v", err)
	}
	msg := emptyResp.Choices[0].Message
	if msg.Content == nil || msg.Content.String == nil {
		t.Fatal("expected non-nil Content with non-nil String for empty content")
	}
	if *msg.Content.String != "" {
		t.Errorf("expected empty string, got '%s'", *msg.Content.String)
	}
	outEmpty, _ := json.Marshal(emptyResp)
	var emptyResp2 ChatCompletion
	json.Unmarshal(outEmpty, &emptyResp2)
	c2 := emptyResp2.Choices[0].Message.Content
	if c2 == nil || c2.String == nil || *c2.String != "" {
		t.Errorf("roundtrip: expected empty string content, got %+v", c2)
	}
}
