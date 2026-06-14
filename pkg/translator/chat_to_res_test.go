package translator

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/tianyuansun/ai-gateway/pkg/schema/chat"
	"github.com/tianyuansun/ai-gateway/pkg/schema/responses"
	"github.com/tianyuansun/ai-gateway/pkg/session"
)

func chatRequest(t *testing.T, model string, messages []chat.ChatCompletionMessage) []byte {
	t.Helper()
	b, err := json.Marshal(chat.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
		Stream:   true,
	})
	if err != nil {
		t.Fatalf("marshal chat request: %v", err)
	}
	return b
}

func TestChatToRes_TranslateRequest_BasicMessages(t *testing.T) {
	// Build a chat request with system + user messages.
	body := chatRequest(t, "gpt-4o", []chat.ChatCompletionMessage{
		{Role: "system", Content: &chat.ChatCompletionMessageContent{String: strPtr("You are helpful.")}},
		{Role: "user", Content: &chat.ChatCompletionMessageContent{String: strPtr("Hello")}},
	})

	tr := &ChatToRes{}
	req := &Request{
		Model:     "gpt-4o",
		APIFormat: FormatChat,
		Body:      body,
	}

	upReq, err := tr.TranslateRequest(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("TranslateRequest: %v", err)
	}

	if upReq.Method != "POST" {
		t.Errorf("expected method POST, got %s", upReq.Method)
	}
	if upReq.URL != "/responses" {
		t.Errorf("expected URL /responses, got %s", upReq.URL)
	}

	var respReq responses.ResponseRequest
	if err := json.Unmarshal(upReq.Body, &respReq); err != nil {
		t.Fatalf("unmarshal response request: %v", err)
	}

	// System message should become instructions.
	if respReq.Instructions == nil || *respReq.Instructions != "You are helpful." {
		t.Errorf("expected instructions 'You are helpful.', got %v", respReq.Instructions)
	}

	// Input should have one user message.
	if respReq.Input.Items == nil || len(respReq.Input.Items) != 1 {
		t.Fatalf("expected 1 input item, got %d", len(respReq.Input.Items))
	}
	if respReq.Input.Items[0].Type != "message" {
		t.Errorf("expected input item type 'message', got %q", respReq.Input.Items[0].Type)
	}
	if respReq.Input.Items[0].Role != "user" {
		t.Errorf("expected input item role 'user', got %q", respReq.Input.Items[0].Role)
	}

	// Check stream is propagated.
	if !respReq.Stream {
		t.Error("expected stream=true")
	}
}

func TestChatToRes_TranslateRequest_ToolCalls(t *testing.T) {
	temperature := 0.5
	topP := 0.9
	maxTokens := 100

	body, err := json.Marshal(chat.ChatCompletionRequest{
		Model: "gpt-4o",
		Messages: []chat.ChatCompletionMessage{
			{Role: "user", Content: &chat.ChatCompletionMessageContent{String: strPtr("Call my function")}},
		},
		Tools: []chat.ChatCompletionTool{
			{
				Type: "function",
				Function: chat.FunctionDefinition{
					Name:        "get_weather",
					Description: "Gets the weather",
					Parameters:  json.RawMessage(`{"type":"object"}`),
				},
			},
		},
		Temperature: &temperature,
		TopP:        &topP,
		MaxTokens:   &maxTokens,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	tr := &ChatToRes{}
	req := &Request{Model: "gpt-4o", APIFormat: FormatChat, Body: body}
	upReq, err := tr.TranslateRequest(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("TranslateRequest: %v", err)
	}

	var respReq responses.ResponseRequest
	if err := json.Unmarshal(upReq.Body, &respReq); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(respReq.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(respReq.Tools))
	}
	if respReq.Tools[0].Name != "get_weather" {
		t.Errorf("expected tool name 'get_weather', got %q", respReq.Tools[0].Name)
	}
	if respReq.Temperature == nil || *respReq.Temperature != 0.5 {
		t.Errorf("expected temperature 0.5, got %v", respReq.Temperature)
	}
	if respReq.TopP == nil || *respReq.TopP != 0.9 {
		t.Errorf("expected top_p 0.9, got %v", respReq.TopP)
	}
	if respReq.MaxOutputTokens == nil || *respReq.MaxOutputTokens != 100 {
		t.Errorf("expected max_output_tokens 100, got %v", respReq.MaxOutputTokens)
	}
}

func TestChatToRes_TranslateRequest_ReasoningEffort(t *testing.T) {
	effort := "high"
	body, err := json.Marshal(chat.ChatCompletionRequest{
		Model: "o4-mini",
		Messages: []chat.ChatCompletionMessage{
			{Role: "user", Content: &chat.ChatCompletionMessageContent{String: strPtr("Think hard")}},
		},
		ReasoningEffort: &effort,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	tr := &ChatToRes{}
	req := &Request{Model: "o4-mini", APIFormat: FormatChat, Body: body}
	upReq, err := tr.TranslateRequest(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("TranslateRequest: %v", err)
	}

	var respReq responses.ResponseRequest
	if err := json.Unmarshal(upReq.Body, &respReq); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if respReq.Reasoning == nil || respReq.Reasoning.Effort == nil || *respReq.Reasoning.Effort != "high" {
		t.Errorf("expected reasoning.effort='high', got %+v", respReq.Reasoning)
	}
}

func TestChatToRes_TranslateRequest_AssistantToolCalls(t *testing.T) {
	body, err := json.Marshal(chat.ChatCompletionRequest{
		Model: "gpt-4o",
		Messages: []chat.ChatCompletionMessage{
			{Role: "user", Content: &chat.ChatCompletionMessageContent{String: strPtr("Call get_weather")}},
			{
				Role:    "assistant",
				Content: nil,
				ToolCalls: []chat.ChatCompletionMessageToolCall{
					{
						ID:   "call_1",
						Type: "function",
						Function: chat.ChatCompletionToolCallFunction{
							Name:      "get_weather",
							Arguments: `{"city":"NYC"}`,
						},
					},
				},
			},
			{
				Role:       "tool",
				ToolCallID: "call_1",
				Content:    &chat.ChatCompletionMessageContent{String: strPtr("72F sunny")},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	tr := &ChatToRes{}
	req := &Request{Model: "gpt-4o", APIFormat: FormatChat, Body: body}
	upReq, err := tr.TranslateRequest(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("TranslateRequest: %v", err)
	}

	var respReq responses.ResponseRequest
	if err := json.Unmarshal(upReq.Body, &respReq); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(respReq.Input.Items) != 3 {
		t.Fatalf("expected 3 input items, got %d", len(respReq.Input.Items))
	}

	// User message
	if respReq.Input.Items[0].Type != "message" || respReq.Input.Items[0].Role != "user" {
		t.Errorf("item 0: expected user message, got type=%q role=%q", respReq.Input.Items[0].Type, respReq.Input.Items[0].Role)
	}

	// Assistant tool call → function_call
	if respReq.Input.Items[1].Type != "function_call" {
		t.Errorf("item 1: expected function_call, got %q", respReq.Input.Items[1].Type)
	}
	if respReq.Input.Items[1].CallID != "call_1" {
		t.Errorf("item 1: expected call_id 'call_1', got %q", respReq.Input.Items[1].CallID)
	}
	if respReq.Input.Items[1].Name != "get_weather" {
		t.Errorf("item 1: expected name 'get_weather', got %q", respReq.Input.Items[1].Name)
	}
	if respReq.Input.Items[1].Arguments != `{"city":"NYC"}` {
		t.Errorf("item 1: expected arguments, got %q", respReq.Input.Items[1].Arguments)
	}

	// Tool response → function_call_output
	if respReq.Input.Items[2].Type != "function_call_output" {
		t.Errorf("item 2: expected function_call_output, got %q", respReq.Input.Items[2].Type)
	}
	if respReq.Input.Items[2].CallID != "call_1" {
		t.Errorf("item 2: expected call_id 'call_1', got %q", respReq.Input.Items[2].CallID)
	}
	if respReq.Input.Items[2].Output != "72F sunny" {
		t.Errorf("item 2: expected output '72F sunny', got %q", respReq.Input.Items[2].Output)
	}
}

// ---------------------------------------------------------------------------
// Cycle 2: TranslateResponse tests
// ---------------------------------------------------------------------------

func TestChatToRes_TranslateResponse_TextMessage(t *testing.T) {
	responsesBody, _ := json.Marshal(responses.Response{
		ID:     "resp_1",
		Object: "response",
		Model:  "gpt-4o",
		Output: []responses.ResponseOutputItem{
			{
				Type: "message",
				Role: "assistant",
				Content: []responses.ResponseContentPart{
					{Type: "output_text", Text: "Hello from responses"},
				},
			},
		},
		Usage: &responses.ResponseUsage{
			InputTokens:  10,
			OutputTokens: 5,
			TotalTokens:  15,
		},
	})

	httpResp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader(responsesBody)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}

	tr := &ChatToRes{}
	req := &Request{Model: "gpt-4o", APIFormat: FormatChat}
	result, err := tr.TranslateResponse(context.Background(), httpResp, req, nil)
	if err != nil {
		t.Fatalf("TranslateResponse: %v", err)
	}
	if result.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", result.StatusCode)
	}

	var chatResp chat.ChatCompletion
	if err := json.Unmarshal(result.Body, &chatResp); err != nil {
		t.Fatalf("unmarshal chat response: %v", err)
	}

	if chatResp.ID != "resp_1" {
		t.Errorf("expected id 'resp_1', got %q", chatResp.ID)
	}
	if len(chatResp.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(chatResp.Choices))
	}
	if chatResp.Choices[0].Message.Content == nil || chatResp.Choices[0].Message.Content.String == nil {
		t.Fatal("expected content")
	}
	if *chatResp.Choices[0].Message.Content.String != "Hello from responses" {
		t.Errorf("expected 'Hello from responses', got %q", *chatResp.Choices[0].Message.Content.String)
	}
	if chatResp.Usage == nil || chatResp.Usage.TotalTokens != 15 {
		t.Errorf("expected total_tokens 15, got %+v", chatResp.Usage)
	}
}

func TestChatToRes_TranslateResponse_FunctionCall(t *testing.T) {
	responsesBody, _ := json.Marshal(responses.Response{
		ID:     "resp_2",
		Object: "response",
		Model:  "gpt-4o",
		Output: []responses.ResponseOutputItem{
			{
				Type:      "function_call",
				CallID:    "fc_1",
				Name:      "get_weather",
				Arguments: `{"city":"NYC"}`,
			},
		},
	})

	httpResp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader(responsesBody)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}

	tr := &ChatToRes{}
	result, err := tr.TranslateResponse(context.Background(), httpResp, nil, nil)
	if err != nil {
		t.Fatalf("TranslateResponse: %v", err)
	}

	var chatResp chat.ChatCompletion
	if err := json.Unmarshal(result.Body, &chatResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	msg := chatResp.Choices[0].Message
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].ID != "fc_1" {
		t.Errorf("expected tool call id 'fc_1', got %q", msg.ToolCalls[0].ID)
	}
	if msg.ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("expected function name 'get_weather', got %q", msg.ToolCalls[0].Function.Name)
	}
	if msg.ToolCalls[0].Function.Arguments != `{"city":"NYC"}` {
		t.Errorf("expected arguments, got %q", msg.ToolCalls[0].Function.Arguments)
	}
}

func TestChatToRes_TranslateResponse_SessionUpdate(t *testing.T) {
	s := &session.Session{ID: "sess-1"}
	responsesBody, _ := json.Marshal(responses.Response{
		ID:     "resp_3",
		Object: "response",
		Model:  "gpt-4o",
		Output: []responses.ResponseOutputItem{
			{
				Type: "message",
				Role: "assistant",
				Content: []responses.ResponseContentPart{
					{Type: "output_text", Text: "session text"},
				},
			},
		},
	})

	httpResp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader(responsesBody)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}

	tr := &ChatToRes{}
	_, err := tr.TranslateResponse(context.Background(), httpResp, nil, s)
	if err != nil {
		t.Fatalf("TranslateResponse: %v", err)
	}

	if len(s.Messages) != 1 {
		t.Fatalf("expected 1 session message, got %d", len(s.Messages))
	}
	if s.Messages[0].Role != "assistant" {
		t.Errorf("expected role assistant, got %q", s.Messages[0].Role)
	}
	if s.Messages[0].Content != "session text" {
		t.Errorf("expected 'session text', got %q", s.Messages[0].Content)
	}
}

func TestChatToRes_UpdateSession_Reasoning(t *testing.T) {
	tr := &ChatToRes{}
	s := &session.Session{ID: "sess-1"}
	// Simulate a response with reasoning content from extractReasoningFromResponse.
	resp := &Response{
		StatusCode:       200,
		Body:             []byte(`{}`),
		ReasoningContent: "Let me think... step by step.",
	}
	tr.UpdateSession(s, nil, resp)
	if len(s.ReasoningRecords) != 1 {
		t.Fatalf("expected 1 reasoning record, got %d", len(s.ReasoningRecords))
	}
	if s.ReasoningRecords[0].Content != "Let me think... step by step." {
		t.Errorf("expected reasoning content, got %q", s.ReasoningRecords[0].Content)
	}
}

// ---------------------------------------------------------------------------
// Cycle 3: TranslateStream test
// ---------------------------------------------------------------------------

func TestChatToRes_TranslateStream(t *testing.T) {
	sseStream := strings.Join([]string{
		`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"delta":{"role":"assistant"},"index":0}]}`,
		``,
		`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"delta":{"content":"Hello"},"index":0}]}`,
		``,
		`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"delta":{"content":" world"},"index":0}]}`,
		``,
		`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"delta":{},"finish_reason":"stop","index":0}]}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	tr := &ChatToRes{}
	ch := tr.TranslateStream(context.Background(), strings.NewReader(sseStream), nil, nil)

	var events []SSEEvent
	for ev := range ch {
		events = append(events, ev)
	}

	if len(events) < 3 {
		t.Fatalf("expected at least 3 events, got %d", len(events))
	}

	// First event should indicate response start.
	if events[0].Event != "response.created" {
		t.Errorf("event 0: expected 'response.created', got %q", events[0].Event)
	}

	// Content delta events should contain the text.
	var allText string
	for _, ev := range events {
		if ev.Event == "response.output_text.delta" {
			var delta struct {
				Delta string `json:"delta"`
			}
			if err := json.Unmarshal(ev.Data, &delta); err == nil {
				allText += delta.Delta
			}
		}
	}
	if allText != "Hello world" {
		t.Errorf("expected accumulated text 'Hello world', got %q", allText)
	}

	// Last event should be completed.
	last := events[len(events)-1]
	if last.Event != "response.completed" {
		t.Errorf("last event: expected 'response.completed', got %q", last.Event)
	}

	// Verify sequence numbers are monotonically increasing.
	var lastSeq int64
	for _, ev := range events {
		var seq struct {
			SequenceNumber int64 `json:"sequence_number"`
		}
		if err := json.Unmarshal(ev.Data, &seq); err != nil {
			continue
		}
		if seq.SequenceNumber <= lastSeq {
			t.Errorf("sequence numbers not monotonic: %d <= %d", seq.SequenceNumber, lastSeq)
		}
		lastSeq = seq.SequenceNumber
	}
}

// ---------------------------------------------------------------------------
// Cycle 4: Edge case tests
// ---------------------------------------------------------------------------

func TestChatToRes_TranslateRequest_EmptyBody(t *testing.T) {
	tr := &ChatToRes{}
	req := &Request{Model: "gpt-4o", APIFormat: FormatChat, Body: []byte(`{}`)}
	upReq, err := tr.TranslateRequest(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("TranslateRequest should not error on empty body: %v", err)
	}
	var respReq responses.ResponseRequest
	if err := json.Unmarshal(upReq.Body, &respReq); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if respReq.Model != "gpt-4o" {
		t.Errorf("expected model 'gpt-4o', got %q", respReq.Model)
	}
}

func TestChatToRes_TranslateRequest_MalformedJSON(t *testing.T) {
	tr := &ChatToRes{}
	req := &Request{Model: "gpt-4o", APIFormat: FormatChat, Body: []byte(`not json`)}
	_, err := tr.TranslateRequest(context.Background(), req, nil)
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestChatToRes_TranslateRequest_SystemOnly(t *testing.T) {
	body := chatRequest(t, "gpt-4o", []chat.ChatCompletionMessage{
		{Role: "system", Content: &chat.ChatCompletionMessageContent{String: strPtr("Instructions only")}},
	})

	tr := &ChatToRes{}
	req := &Request{Model: "gpt-4o", APIFormat: FormatChat, Body: body}
	upReq, err := tr.TranslateRequest(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("TranslateRequest: %v", err)
	}

	var respReq responses.ResponseRequest
	if err := json.Unmarshal(upReq.Body, &respReq); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if respReq.Instructions == nil || *respReq.Instructions != "Instructions only" {
		t.Errorf("expected instructions, got %v", respReq.Instructions)
	}
}

func TestChatToRes_TranslateRequest_AssistantText(t *testing.T) {
	body := chatRequest(t, "gpt-4o", []chat.ChatCompletionMessage{
		{Role: "user", Content: &chat.ChatCompletionMessageContent{String: strPtr("What is 2+2?")}},
		{Role: "assistant", Content: &chat.ChatCompletionMessageContent{String: strPtr("The answer is 4.")}},
	})

	tr := &ChatToRes{}
	req := &Request{Model: "gpt-4o", APIFormat: FormatChat, Body: body}
	upReq, err := tr.TranslateRequest(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("TranslateRequest: %v", err)
	}

	var respReq responses.ResponseRequest
	if err := json.Unmarshal(upReq.Body, &respReq); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(respReq.Input.Items) != 2 {
		t.Fatalf("expected 2 input items, got %d", len(respReq.Input.Items))
	}
	if respReq.Input.Items[1].Type != "message" || respReq.Input.Items[1].Role != "assistant" {
		t.Errorf("item 1: expected assistant message, got type=%q role=%q", respReq.Input.Items[1].Type, respReq.Input.Items[1].Role)
	}

	// Verify the assistant content is preserved.
	var content string
	if err := json.Unmarshal(respReq.Input.Items[1].Content, &content); err == nil {
		if content != "The answer is 4." {
			t.Errorf("expected 'The answer is 4.', got %q", content)
		}
	}
}

func TestChatToRes_TranslateStream_EmptyStream(t *testing.T) {
	tr := &ChatToRes{}
	ch := tr.TranslateStream(context.Background(), strings.NewReader(""), nil, nil)
	var events []SSEEvent
	for ev := range ch {
		events = append(events, ev)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events for empty stream, got %d", len(events))
	}
}

func TestChatToRes_TranslateResponse_EmptyBody(t *testing.T) {
	tr := &ChatToRes{}
	httpResp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader([]byte{})),
	}
	_, err := tr.TranslateResponse(context.Background(), httpResp, nil, nil)
	if err == nil {
		t.Error("expected error for empty body")
	}
}

func TestChatToRes_UpdateSession_NoReasoning(t *testing.T) {
	tr := &ChatToRes{}
	s := &session.Session{ID: "sess-1"}
	resp := &Response{StatusCode: 200, Body: []byte(`{}`), ReasoningContent: ""}
	tr.UpdateSession(s, nil, resp)
	if len(s.ReasoningRecords) != 0 {
		t.Errorf("expected 0 reasoning records, got %d", len(s.ReasoningRecords))
	}
}
