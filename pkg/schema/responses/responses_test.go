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
