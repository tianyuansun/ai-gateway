package responses

import (
	"encoding/json"
	"testing"
)

func TestResponseRoundtripWithOutputMessage(t *testing.T) {
	respJSON := `{
		"id": "resp_abc123",
		"object": "response",
		"created_at": 1741476542,
		"status": "completed",
		"model": "gpt-4o",
		"output": [
			{
				"type": "message",
				"id": "msg_abc123",
				"status": "completed",
				"role": "assistant",
				"content": [
					{
						"type": "output_text",
						"text": "Hello! How can I help you?",
						"annotations": []
					}
				]
			}
		],
		"usage": {
			"input_tokens": 36,
			"output_tokens": 87,
			"total_tokens": 123
		}
	}`

	var resp Response
	if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.ID != "resp_abc123" {
		t.Errorf("expected id resp_abc123, got %s", resp.ID)
	}
	if resp.Object != "response" {
		t.Errorf("expected object response, got %s", resp.Object)
	}
	if resp.Status != "completed" {
		t.Errorf("expected status completed, got %s", resp.Status)
	}
	if resp.Model != "gpt-4o" {
		t.Errorf("expected model gpt-4o, got %s", resp.Model)
	}
	if len(resp.Output) != 1 {
		t.Fatalf("expected 1 output item, got %d", len(resp.Output))
	}
	if resp.Output[0].Type != "message" {
		t.Errorf("expected output[0].type message, got %s", resp.Output[0].Type)
	}
	if resp.Output[0].ID != "msg_abc123" {
		t.Errorf("expected output[0].id msg_abc123, got %s", resp.Output[0].ID)
	}
	if resp.Output[0].Status != "completed" {
		t.Errorf("expected output[0].status completed, got %s", resp.Output[0].Status)
	}
	if resp.Output[0].Role != "assistant" {
		t.Errorf("expected output[0].role assistant, got %s", resp.Output[0].Role)
	}
	if len(resp.Output[0].Content) != 1 {
		t.Fatalf("expected 1 content part, got %d", len(resp.Output[0].Content))
	}
	if resp.Output[0].Content[0].Type != "output_text" {
		t.Errorf("expected content[0].type output_text, got %s", resp.Output[0].Content[0].Type)
	}
	if resp.Output[0].Content[0].Text != "Hello! How can I help you?" {
		t.Errorf("unexpected content text: %s", resp.Output[0].Content[0].Text)
	}
	if resp.Usage == nil {
		t.Fatal("expected usage, got nil")
	}
	if resp.Usage.InputTokens != 36 {
		t.Errorf("expected input_tokens 36, got %d", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens != 87 {
		t.Errorf("expected output_tokens 87, got %d", resp.Usage.OutputTokens)
	}
	if resp.Usage.TotalTokens != 123 {
		t.Errorf("expected total_tokens 123, got %d", resp.Usage.TotalTokens)
	}

	// Roundtrip: marshal back.
	out, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	var resp2 Response
	if err := json.Unmarshal(out, &resp2); err != nil {
		t.Fatalf("unmarshal roundtrip: %v", err)
	}
	if resp2.ID != "resp_abc123" {
		t.Errorf("roundtrip id mismatch: got %s", resp2.ID)
	}
	if resp2.Usage.TotalTokens != 123 {
		t.Errorf("roundtrip total_tokens mismatch: got %d", resp2.Usage.TotalTokens)
	}
}

func TestResponseRoundtripWithFunctionCall(t *testing.T) {
	respJSON := `{
		"id": "resp_func123",
		"object": "response",
		"created_at": 1741476542,
		"status": "completed",
		"model": "gpt-4o",
		"output": [
			{
				"type": "function_call",
				"id": "fc_abc123",
				"call_id": "call_abc123",
				"name": "get_weather",
				"arguments": "{\"location\":\"San Francisco\"}"
			}
		],
		"usage": {
			"input_tokens": 50,
			"output_tokens": 30,
			"total_tokens": 80
		}
	}`

	var resp Response
	if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.ID != "resp_func123" {
		t.Errorf("expected id resp_func123, got %s", resp.ID)
	}
	if len(resp.Output) != 1 {
		t.Fatalf("expected 1 output item, got %d", len(resp.Output))
	}
	if resp.Output[0].Type != "function_call" {
		t.Errorf("expected output[0].type function_call, got %s", resp.Output[0].Type)
	}
	if resp.Output[0].Name != "get_weather" {
		t.Errorf("expected function name get_weather, got %s", resp.Output[0].Name)
	}
	if resp.Output[0].Arguments != `{"location":"San Francisco"}` {
		t.Errorf("unexpected arguments: %s", resp.Output[0].Arguments)
	}

	// Roundtrip.
	out, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	var resp2 Response
	if err := json.Unmarshal(out, &resp2); err != nil {
		t.Fatalf("unmarshal roundtrip: %v", err)
	}
	if resp2.ID != "resp_func123" {
		t.Errorf("roundtrip id mismatch: got %s", resp2.ID)
	}
	if resp2.Output[0].Name != "get_weather" {
		t.Errorf("roundtrip function name mismatch: got %s", resp2.Output[0].Name)
	}
}

func TestInputItemsMessageRoundtrip(t *testing.T) {
	// Test request with message input items (content as string).
	reqJSON := `{
		"model": "gpt-4o",
		"input": [
			{
				"type": "message",
				"role": "user",
				"content": "Hello!"
			}
		]
	}`

	var req ResponseRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if req.Model != "gpt-4o" {
		t.Errorf("expected model gpt-4o, got %s", req.Model)
	}
	if req.Input.String != nil {
		t.Errorf("expected Input.String to be nil for array input")
	}
	if len(req.Input.Items) != 1 {
		t.Fatalf("expected 1 input item, got %d", len(req.Input.Items))
	}
	if req.Input.Items[0].Type != "message" {
		t.Errorf("expected input[0].type message, got %s", req.Input.Items[0].Type)
	}
	if req.Input.Items[0].Role != "user" {
		t.Errorf("expected input[0].role user, got %s", req.Input.Items[0].Role)
	}

	// Roundtrip.
	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	var req2 ResponseRequest
	if err := json.Unmarshal(out, &req2); err != nil {
		t.Fatalf("unmarshal roundtrip: %v", err)
	}
	if req2.Model != "gpt-4o" {
		t.Errorf("roundtrip model mismatch: got %s", req2.Model)
	}
	if req2.Input.String != nil {
		t.Errorf("roundtrip: expected Input.String to be nil")
	}
	if len(req2.Input.Items) != 1 {
		t.Fatalf("roundtrip: expected 1 input item, got %d", len(req2.Input.Items))
	}
	if req2.Input.Items[0].Role != "user" {
		t.Errorf("roundtrip role mismatch: got %s", req2.Input.Items[0].Role)
	}
}

func TestResponseContentPartRefusal(t *testing.T) {
	// Content parts can also include refusal.
	respJSON := `{
		"id": "resp_refusal",
		"object": "response",
		"created_at": 1741476542,
		"status": "completed",
		"model": "gpt-4o",
		"output": [
			{
				"type": "message",
				"id": "msg_refusal",
				"status": "completed",
				"role": "assistant",
				"content": [
					{
						"type": "refusal",
						"refusal": "I cannot answer that."
					}
				]
			}
		],
		"usage": {
			"input_tokens": 10,
			"output_tokens": 5,
			"total_tokens": 15
		}
	}`

	var resp Response
	if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
		t.Fatalf("unmarshal refusal response: %v", err)
	}
	if len(resp.Output[0].Content) != 1 {
		t.Fatalf("expected 1 content part, got %d", len(resp.Output[0].Content))
	}
	if resp.Output[0].Content[0].Type != "refusal" {
		t.Errorf("expected content type refusal, got %s", resp.Output[0].Content[0].Type)
	}
	if resp.Output[0].Content[0].Refusal != "I cannot answer that." {
		t.Errorf("unexpected refusal text: %s", resp.Output[0].Content[0].Refusal)
	}
}

func TestResponseStreamEventTypes(t *testing.T) {
	// Test that streaming events parse correctly.
	eventsJSON := []string{
		`{"type": "response.created", "response": {"id": "resp_evt", "object": "response", "created_at": 1, "status": "in_progress", "model": "gpt-4o", "output": []}}`,
		`{"type": "response.output_item.added", "item": {"type": "message", "id": "msg_evt", "status": "in_progress", "role": "assistant", "content": []}}`,
		`{"type": "response.content_part.added", "content_part": {"type": "output_text", "text": "", "annotations": []}}`,
		`{"type": "response.output_text.delta", "delta": "Hello"}`,
		`{"type": "response.output_text.done", "text": "Hello, world!"}`,
		`{"type": "response.completed", "response": {"id": "resp_evt", "object": "response", "created_at": 1, "status": "completed", "model": "gpt-4o", "output": []}}`,
		`{"type": "error", "message": "Something went wrong", "code": "server_error"}`,
	}

	for i, ej := range eventsJSON {
		var evt ResponseStreamEvent
		if err := json.Unmarshal([]byte(ej), &evt); err != nil {
			t.Fatalf("event %d: unmarshal error: %v", i, err)
		}
		// Roundtrip each event.
		out, err := json.Marshal(evt)
		if err != nil {
			t.Fatalf("event %d: marshal error: %v", i, err)
		}
		var evt2 ResponseStreamEvent
		if err := json.Unmarshal(out, &evt2); err != nil {
			t.Fatalf("event %d: roundtrip unmarshal error: %v", i, err)
		}
		if evt2.Type != evt.Type {
			t.Errorf("event %d: roundtrip type mismatch: %s vs %s", i, evt.Type, evt2.Type)
		}
	}
}

// -- Deep roundtrip tests for Issue #52 -------------------------------------------

// TestFunctionCallOutputItemRoundtrip verifies a request with a
// function_call_output input item survives a roundtrip.
func TestFunctionCallOutputItemRoundtrip(t *testing.T) {
	reqJSON := `{
		"model": "gpt-4o",
		"input": [
			{
				"type": "message",
				"role": "user",
				"content": "What is the weather in SF?"
			},
			{
				"type": "function_call",
				"call_id": "call_abc123",
				"name": "get_weather",
				"arguments": "{\"location\":\"San Francisco\"}",
				"id": "fc_abc123"
			},
			{
				"type": "function_call_output",
				"call_id": "call_abc123",
				"output": "{\"temperature\": 22, \"condition\": \"sunny\"}"
			}
		]
	}`

	var req ResponseRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		t.Fatalf("unmarshal function_call_output request: %v", err)
	}
	if req.Model != "gpt-4o" {
		t.Errorf("model mismatch")
	}
	if len(req.Input.Items) != 3 {
		t.Fatalf("expected 3 input items, got %d", len(req.Input.Items))
	}

	// Check function_call item.
	fc := req.Input.Items[1]
	if fc.Type != "function_call" {
		t.Errorf("expected item[1] type=function_call, got %s", fc.Type)
	}
	if fc.CallID != "call_abc123" {
		t.Errorf("function_call call_id mismatch")
	}
	if fc.Name != "get_weather" {
		t.Errorf("function_call name mismatch")
	}

	// Check function_call_output item.
	fco := req.Input.Items[2]
	if fco.Type != "function_call_output" {
		t.Errorf("expected item[2] type=function_call_output, got %s", fco.Type)
	}
	if fco.CallID != "call_abc123" {
		t.Errorf("function_call_output call_id mismatch")
	}
	if fco.Output != `{"temperature": 22, "condition": "sunny"}` {
		t.Errorf("function_call_output output mismatch: %s", fco.Output)
	}

	// Roundtrip.
	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal function_call_output request: %v", err)
	}
	var req2 ResponseRequest
	if err := json.Unmarshal(out, &req2); err != nil {
		t.Fatalf("unmarshal function_call_output roundtrip: %v", err)
	}
	if len(req2.Input.Items) != 3 {
		t.Fatalf("roundtrip: expected 3 input items, got %d", len(req2.Input.Items))
	}
	fco2 := req2.Input.Items[2]
	if fco2.Type != "function_call_output" {
		t.Errorf("roundtrip: type mismatch")
	}
	if fco2.CallID != "call_abc123" {
		t.Errorf("roundtrip: call_id mismatch")
	}
	if fco2.Output != `{"temperature": 22, "condition": "sunny"}` {
		t.Errorf("roundtrip: output mismatch")
	}
}

// TestRefusalContentPartDeepRoundtrip verifies refusal content parts survive
// a full marshal-unmarshal roundtrip.
func TestRefusalContentPartDeepRoundtrip(t *testing.T) {
	respJSON := `{
		"id": "resp_refusal_deep",
		"object": "response",
		"created_at": 1741476542,
		"status": "completed",
		"model": "gpt-4o",
		"output": [
			{
				"type": "message",
				"id": "msg_refusal_deep",
				"status": "completed",
				"role": "assistant",
				"content": [
					{
						"type": "refusal",
						"refusal": "I'm sorry, I cannot provide instructions for that activity."
					}
				]
			}
		],
		"usage": {
			"input_tokens": 25,
			"output_tokens": 12,
			"total_tokens": 37
		}
	}`

	var resp Response
	if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
		t.Fatalf("unmarshal refusal deep response: %v", err)
	}
	outItem := resp.Output[0]
	if outItem.Type != "message" {
		t.Errorf("expected output type=message, got %s", outItem.Type)
	}
	if len(outItem.Content) != 1 {
		t.Fatalf("expected 1 content part, got %d", len(outItem.Content))
	}
	cp := outItem.Content[0]
	if cp.Type != "refusal" {
		t.Errorf("expected content type=refusal, got %s", cp.Type)
	}
	if cp.Refusal != "I'm sorry, I cannot provide instructions for that activity." {
		t.Errorf("refusal text mismatch")
	}

	// Roundtrip.
	out, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal refusal deep response: %v", err)
	}
	var resp2 Response
	if err := json.Unmarshal(out, &resp2); err != nil {
		t.Fatalf("unmarshal refusal deep roundtrip: %v", err)
	}
	cp2 := resp2.Output[0].Content[0]
	if cp2.Type != "refusal" {
		t.Errorf("roundtrip: expected type=refusal, got %s", cp2.Type)
	}
	if cp2.Refusal != "I'm sorry, I cannot provide instructions for that activity." {
		t.Errorf("roundtrip: refusal text mismatch")
	}
}

// TestMultipleOutputItemsRoundtrip verifies a response with multiple output items
// (function_call + message) survives a roundtrip.
func TestMultipleOutputItemsRoundtrip(t *testing.T) {
	respJSON := `{
		"id": "resp_multi_out",
		"object": "response",
		"created_at": 1741476542,
		"status": "completed",
		"model": "gpt-4o",
		"output": [
			{
				"type": "function_call",
				"id": "fc_abc",
				"call_id": "call_abc",
				"name": "get_weather",
				"arguments": "{\"location\":\"San Francisco\"}"
			},
			{
				"type": "message",
				"id": "msg_abc",
				"status": "completed",
				"role": "assistant",
				"content": [
					{
						"type": "output_text",
						"text": "The weather in San Francisco is sunny with a temperature of 22C.",
						"annotations": []
					}
				]
			}
		],
		"usage": {
			"input_tokens": 80,
			"output_tokens": 120,
			"total_tokens": 200
		}
	}`

	var resp Response
	if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
		t.Fatalf("unmarshal multiple output items: %v", err)
	}
	if len(resp.Output) != 2 {
		t.Fatalf("expected 2 output items, got %d", len(resp.Output))
	}
	if resp.Output[0].Type != "function_call" {
		t.Errorf("output[0] type mismatch: got %s", resp.Output[0].Type)
	}
	if resp.Output[0].Name != "get_weather" {
		t.Errorf("output[0] name mismatch")
	}
	if resp.Output[1].Type != "message" {
		t.Errorf("output[1] type mismatch: got %s", resp.Output[1].Type)
	}
	if resp.Output[1].Role != "assistant" {
		t.Errorf("output[1] role mismatch")
	}
	if len(resp.Output[1].Content) != 1 {
		t.Fatalf("expected 1 content part in message, got %d", len(resp.Output[1].Content))
	}

	// Roundtrip.
	out, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal multiple output items: %v", err)
	}
	var resp2 Response
	if err := json.Unmarshal(out, &resp2); err != nil {
		t.Fatalf("unmarshal multiple output items roundtrip: %v", err)
	}
	if len(resp2.Output) != 2 {
		t.Fatalf("roundtrip: expected 2 output items, got %d", len(resp2.Output))
	}
	if resp2.Output[0].Type != "function_call" {
		t.Errorf("roundtrip: output[0] type mismatch")
	}
	if resp2.Output[0].Name != "get_weather" {
		t.Errorf("roundtrip: output[0] name mismatch")
	}
	if resp2.Output[1].Type != "message" {
		t.Errorf("roundtrip: output[1] type mismatch")
	}
	if resp2.Usage.TotalTokens != 200 {
		t.Errorf("roundtrip: total_tokens mismatch")
	}
}

// TestStreamEventErrorDeepRoundtrip verifies the error stream event survives
// a full roundtrip with all error-specific fields checked.
func TestStreamEventErrorDeepRoundtrip(t *testing.T) {
	errJSON := `{
		"type": "error",
		"message": "The model produced invalid content that violated the usage policies.",
		"code": "content_filter"
	}`

	var evt ResponseStreamEvent
	if err := json.Unmarshal([]byte(errJSON), &evt); err != nil {
		t.Fatalf("unmarshal error event: %v", err)
	}
	if evt.Type != "error" {
		t.Errorf("expected type=error, got %s", evt.Type)
	}
	if evt.Message != "The model produced invalid content that violated the usage policies." {
		t.Errorf("error message mismatch: %s", evt.Message)
	}
	if evt.Code != "content_filter" {
		t.Errorf("error code mismatch: %s", evt.Code)
	}

	// Roundtrip.
	out, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal error event: %v", err)
	}
	var evt2 ResponseStreamEvent
	if err := json.Unmarshal(out, &evt2); err != nil {
		t.Fatalf("unmarshal error event roundtrip: %v", err)
	}
	if evt2.Type != "error" {
		t.Errorf("roundtrip: type mismatch")
	}
	if evt2.Message != "The model produced invalid content that violated the usage policies." {
		t.Errorf("roundtrip: message mismatch")
	}
	if evt2.Code != "content_filter" {
		t.Errorf("roundtrip: code mismatch")
	}
}
