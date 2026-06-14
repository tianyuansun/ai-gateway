package translator

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/tianyuansun/ai-gateway/pkg/schema/anthropic"
	"github.com/tianyuansun/ai-gateway/pkg/schema/responses"
)

func TestAnthToRes_TranslateRequest_MapsMessagesToInput(t *testing.T) {
	systemText := "You are a helpful assistant."
	anthReq := anthropic.MessageRequest{
		Model: "claude-sonnet-4-5",
		System: &anthropic.SystemContent{
			String: &systemText,
		},
		Messages: []anthropic.MessageParam{
			{
				Role: "user",
				Content: []anthropic.ContentBlockParam{
					{Type: "text", Text: "Hello"},
				},
			},
		},
		MaxTokens: 4096,
	}

	body, _ := json.Marshal(anthReq)
	req := &Request{
		Model:     "claude-sonnet-4-5",
		APIFormat: FormatAnthropic,
		Body:      body,
	}

	tr := &AnthToRes{}
	upReq, err := tr.TranslateRequest(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("TranslateRequest: %v", err)
	}

	if upReq.URL != "/responses" {
		t.Errorf("expected URL /responses, got %q", upReq.URL)
	}

	var resReq responses.ResponseRequest
	if err := json.Unmarshal(upReq.Body, &resReq); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if resReq.Instructions == nil || *resReq.Instructions != "You are a helpful assistant." {
		t.Errorf("expected instructions, got %v", resReq.Instructions)
	}

	if len(resReq.Input.Items) != 1 {
		t.Fatalf("expected 1 input item, got %d", len(resReq.Input.Items))
	}
	if resReq.Input.Items[0].Type != "message" {
		t.Errorf("expected input item type 'message', got %q", resReq.Input.Items[0].Type)
	}
	if resReq.Input.Items[0].Role != "user" {
		t.Errorf("expected input item role 'user', got %q", resReq.Input.Items[0].Role)
	}
}

func TestAnthToRes_TranslateRequest_MapsToolUseToFunctionCall(t *testing.T) {
	anthReq := anthropic.MessageRequest{
		Model: "claude-sonnet-4-5",
		Messages: []anthropic.MessageParam{
			{
				Role: "user",
				Content: []anthropic.ContentBlockParam{
					{Type: "text", Text: "What is 2+2?"},
				},
			},
			{
				Role: "assistant",
				Content: []anthropic.ContentBlockParam{
					{Type: "tool_use", ID: "tool_1", Name: "calculator", Input: json.RawMessage(`{"expr":"2+2"}`)},
				},
			},
			{
				Role: "user",
				Content: []anthropic.ContentBlockParam{
					{Type: "tool_result", ToolUseID: "tool_1", Content: "4"},
				},
			},
		},
		MaxTokens: 4096,
	}

	body, _ := json.Marshal(anthReq)
	req := &Request{
		Model:     "claude-sonnet-4-5",
		APIFormat: FormatAnthropic,
		Body:      body,
	}

	tr := &AnthToRes{}
	upReq, err := tr.TranslateRequest(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("TranslateRequest: %v", err)
	}

	var resReq responses.ResponseRequest
	if err := json.Unmarshal(upReq.Body, &resReq); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if len(resReq.Input.Items) != 3 {
		t.Fatalf("expected 3 input items, got %d", len(resReq.Input.Items))
	}

	// First: user message
	if resReq.Input.Items[0].Type != "message" || resReq.Input.Items[0].Role != "user" {
		t.Errorf("item 0: expected user message, got type=%q role=%q", resReq.Input.Items[0].Type, resReq.Input.Items[0].Role)
	}

	// Second: function_call (from assistant tool_use)
	if resReq.Input.Items[1].Type != "function_call" {
		t.Errorf("item 1: expected 'function_call', got %q", resReq.Input.Items[1].Type)
	}
	if resReq.Input.Items[1].CallID != "tool_1" {
		t.Errorf("item 1: expected call_id 'tool_1', got %q", resReq.Input.Items[1].CallID)
	}
	if resReq.Input.Items[1].Name != "calculator" {
		t.Errorf("item 1: expected name 'calculator', got %q", resReq.Input.Items[1].Name)
	}

	// Third: function_call_output (from tool_result)
	if resReq.Input.Items[2].Type != "function_call_output" {
		t.Errorf("item 2: expected 'function_call_output', got %q", resReq.Input.Items[2].Type)
	}
	if resReq.Input.Items[2].CallID != "tool_1" {
		t.Errorf("item 2: expected call_id 'tool_1', got %q", resReq.Input.Items[2].CallID)
	}
	if resReq.Input.Items[2].Output != "4" {
		t.Errorf("item 2: expected output '4', got %q", resReq.Input.Items[2].Output)
	}
}

func TestAnthToRes_TranslateRequest_MapsThinkingToReasoning(t *testing.T) {
	anthReq := anthropic.MessageRequest{
		Model: "claude-sonnet-4-5",
		Messages: []anthropic.MessageParam{
			{
				Role: "user",
				Content: []anthropic.ContentBlockParam{
					{Type: "text", Text: "Think deeply."},
				},
			},
		},
		MaxTokens: 4096,
		Thinking: &anthropic.ThinkingConfig{
			Type:         "enabled",
			BudgetTokens: 16000,
		},
	}

	body, _ := json.Marshal(anthReq)
	req := &Request{
		Model:     "claude-sonnet-4-5",
		APIFormat: FormatAnthropic,
		Body:      body,
	}

	tr := &AnthToRes{}
	upReq, err := tr.TranslateRequest(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("TranslateRequest: %v", err)
	}

	var resReq responses.ResponseRequest
	if err := json.Unmarshal(upReq.Body, &resReq); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if resReq.Reasoning == nil {
		t.Fatal("expected Reasoning config, got nil")
	}
	if resReq.Reasoning.Effort == nil || *resReq.Reasoning.Effort != "high" {
		t.Errorf("expected reasoning effort 'high', got %v", resReq.Reasoning.Effort)
	}
	if resReq.Reasoning.Summary == nil || *resReq.Reasoning.Summary != "auto" {
		t.Errorf("expected reasoning summary 'auto', got %v", resReq.Reasoning.Summary)
	}
}

func TestAnthToRes_TranslateRequest_MapsTools(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"location":{"type":"string"}}}`)
	anthReq := anthropic.MessageRequest{
		Model: "claude-sonnet-4-5",
		Messages: []anthropic.MessageParam{
			{
				Role: "user",
				Content: []anthropic.ContentBlockParam{
					{Type: "text", Text: "What's the weather?"},
				},
			},
		},
		MaxTokens: 4096,
		Tools: []anthropic.ToolDefinition{
			{
				Name:        "get_weather",
				Description: "Get weather for a location",
				InputSchema: schema,
			},
		},
	}

	body, _ := json.Marshal(anthReq)
	req := &Request{
		Model:     "claude-sonnet-4-5",
		APIFormat: FormatAnthropic,
		Body:      body,
	}

	tr := &AnthToRes{}
	upReq, err := tr.TranslateRequest(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("TranslateRequest: %v", err)
	}

	var resReq responses.ResponseRequest
	if err := json.Unmarshal(upReq.Body, &resReq); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if len(resReq.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(resReq.Tools))
	}
	if resReq.Tools[0].Name != "get_weather" {
		t.Errorf("expected tool name 'get_weather', got %q", resReq.Tools[0].Name)
	}
	if resReq.Tools[0].Description != "Get weather for a location" {
		t.Errorf("expected tool description mismatch, got %q", resReq.Tools[0].Description)
	}
}

// --- TranslateResponse tests ---

func TestAnthToRes_TranslateResponse_MapsOutputText(t *testing.T) {
	resResp := responses.Response{
		ID:     "resp_123",
		Object: "response",
		Status: "completed",
		Model:  "gpt-5",
		Output: []responses.ResponseOutputItem{
			{
				Type: "message",
				Role: "assistant",
				Content: []responses.ResponseContentPart{
					{Type: "output_text", Text: "Hello from responses!"},
				},
			},
		},
		Usage: &responses.ResponseUsage{
			InputTokens:  10,
			OutputTokens: 5,
			TotalTokens:  15,
		},
	}

	respBody, _ := json.Marshal(resResp)
	httpResp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader(respBody)),
	}

	tr := &AnthToRes{}
	req := &Request{
		Model:     "gpt-5",
		APIFormat: FormatAnthropic,
	}

	result, err := tr.TranslateResponse(context.Background(), httpResp, req, nil)
	if err != nil {
		t.Fatalf("TranslateResponse: %v", err)
	}
	if result.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", result.StatusCode)
	}

	var anthResp anthropic.MessageResponse
	if err := json.Unmarshal(result.Body, &anthResp); err != nil {
		t.Fatalf("unmarshal result body: %v", err)
	}
	if anthResp.ID != "resp_123" {
		t.Errorf("expected ID 'resp_123', got %q", anthResp.ID)
	}
	if anthResp.Type != "message" {
		t.Errorf("expected type 'message', got %q", anthResp.Type)
	}
	if anthResp.Role != "assistant" {
		t.Errorf("expected role 'assistant', got %q", anthResp.Role)
	}
	if len(anthResp.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(anthResp.Content))
	}
	if anthResp.Content[0].Type != "text" || anthResp.Content[0].Text != "Hello from responses!" {
		t.Errorf("expected text content 'Hello from responses!', got %+v", anthResp.Content[0])
	}
	if anthResp.Usage.InputTokens != 10 {
		t.Errorf("expected input tokens 10, got %d", anthResp.Usage.InputTokens)
	}
	if anthResp.Usage.OutputTokens != 5 {
		t.Errorf("expected output tokens 5, got %d", anthResp.Usage.OutputTokens)
	}
}

func TestAnthToRes_TranslateResponse_MapsFunctionCall(t *testing.T) {
	resResp := responses.Response{
		ID:     "resp_456",
		Object: "response",
		Status: "completed",
		Model:  "gpt-5",
		Output: []responses.ResponseOutputItem{
			{
				Type:      "function_call",
				CallID:    "call_abc",
				Name:      "get_weather",
				Arguments: `{"location":"NYC"}`,
			},
		},
	}

	respBody, _ := json.Marshal(resResp)
	httpResp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader(respBody)),
	}

	tr := &AnthToRes{}
	req := &Request{
		Model:     "gpt-5",
		APIFormat: FormatAnthropic,
	}

	result, err := tr.TranslateResponse(context.Background(), httpResp, req, nil)
	if err != nil {
		t.Fatalf("TranslateResponse: %v", err)
	}

	var anthResp anthropic.MessageResponse
	if err := json.Unmarshal(result.Body, &anthResp); err != nil {
		t.Fatalf("unmarshal result body: %v", err)
	}
	if len(anthResp.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(anthResp.Content))
	}
	if anthResp.Content[0].Type != "tool_use" {
		t.Errorf("expected type 'tool_use', got %q", anthResp.Content[0].Type)
	}
	if anthResp.Content[0].ID != "call_abc" {
		t.Errorf("expected ID 'call_abc', got %q", anthResp.Content[0].ID)
	}
	if anthResp.Content[0].Name != "get_weather" {
		t.Errorf("expected name 'get_weather', got %q", anthResp.Content[0].Name)
	}
}

func TestAnthToRes_TranslateResponse_ExtractsReasoning(t *testing.T) {
	resResp := responses.Response{
		ID:     "resp_789",
		Object: "response",
		Status: "completed",
		Model:  "gpt-5",
		Output: []responses.ResponseOutputItem{
			{
				Type: "reasoning",
				ID:   "rs_1",
				Summary: []json.RawMessage{
					json.RawMessage(`{"type":"summary_text","text":"Let me think about this step by step."}`),
				},
			},
			{
				Type: "message",
				Role: "assistant",
				Content: []responses.ResponseContentPart{
					{Type: "output_text", Text: "The answer is 42."},
				},
			},
		},
	}

	respBody, _ := json.Marshal(resResp)
	httpResp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader(respBody)),
	}

	tr := &AnthToRes{}
	req := &Request{
		Model:     "gpt-5",
		APIFormat: FormatAnthropic,
	}

	result, err := tr.TranslateResponse(context.Background(), httpResp, req, nil)
	if err != nil {
		t.Fatalf("TranslateResponse: %v", err)
	}

	// Reasoning should be extracted into ReasoningContent field.
	if result.ReasoningContent == "" {
		t.Error("expected non-empty ReasoningContent for reasoning output")
	}

	// The message output should still be in the body.
	var anthResp anthropic.MessageResponse
	if err := json.Unmarshal(result.Body, &anthResp); err != nil {
		t.Fatalf("unmarshal result body: %v", err)
	}
	if len(anthResp.Content) != 1 {
		t.Fatalf("expected 1 content block (message only), got %d", len(anthResp.Content))
	}
	if anthResp.Content[0].Type != "text" || anthResp.Content[0].Text != "The answer is 42." {
		t.Errorf("expected text 'The answer is 42.', got %+v", anthResp.Content[0])
	}
}

// --- TranslateStream tests ---

func TestAnthToRes_TranslateStream_EmitsAnthropicEvents(t *testing.T) {
	// Responses API SSE stream (upstream) → Anthropic SSE events (client).
	resSSE := joinLines(
		`data: {"type":"response.created","response":{"id":"resp_1","object":"response"}}`,
		``,
		`data: {"type":"response.in_progress","response":{"id":"resp_1","object":"response"}}`,
		``,
		`data: {"type":"response.output_item.added","output_index":0,"item":{"id":"resp_1_item","type":"message","role":"assistant","status":"in_progress"}}`,
		``,
		`data: {"type":"response.content_part.added","item_id":"resp_1_item","output_index":0,"content_index":0,"part":{"type":"output_text","text":""}}`,
		``,
		`data: {"type":"response.output_text.delta","item_id":"resp_1_item","output_index":0,"content_index":0,"delta":"Hello"}`,
		``,
		`data: {"type":"response.output_text.delta","item_id":"resp_1_item","output_index":0,"content_index":0,"delta":" world"}`,
		``,
		`data: {"type":"response.content_part.done","item_id":"resp_1_item","output_index":0,"content_index":0,"part":{"type":"output_text","text":"Hello world"}}`,
		``,
		`data: {"type":"response.output_item.done","output_index":0,"item":{"id":"resp_1_item","type":"message","role":"assistant","status":"completed"}}`,
		``,
		`data: {"type":"response.completed","response":{"id":"resp_1","object":"response","usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}}}`,
		``,
	)

	tr := &AnthToRes{}
	ch := tr.TranslateStream(context.Background(), strings.NewReader(resSSE), nil, nil)

	var events []SSEEvent
	for ev := range ch {
		events = append(events, ev)
	}

	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}

	// First event should be message_start.
	if events[0].Event != "message_start" {
		t.Errorf("first event: expected 'message_start', got %q", events[0].Event)
	}

	// Find content_block_delta events and accumulate text.
	var allText string
	for _, ev := range events {
		if ev.Event == "content_block_delta" {
			var delta struct {
				Type  string `json:"type"`
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
			}
			if json.Unmarshal(ev.Data, &delta) == nil && delta.Delta.Type == "text_delta" {
				allText += delta.Delta.Text
			}
		}
	}
	if allText != "Hello world" {
		t.Errorf("expected accumulated text 'Hello world', got %q", allText)
	}

	// Last event should be message_stop.
	last := events[len(events)-1]
	if last.Event != "message_stop" {
		t.Errorf("last event: expected 'message_stop', got %q", last.Event)
	}
}

// joinLines joins strings with newline. Unlike strings.Join, it does not add
// a trailing newline on the last element — but for SSE, we want newlines
// between events so each line is joined with \n.
func joinLines(lines ...string) string {
	return strings.Join(lines, "\n")
}
