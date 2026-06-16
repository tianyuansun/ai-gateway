package translator

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tianyuansun/ai-gateway/pkg/schema/chat"
	"github.com/tianyuansun/ai-gateway/pkg/session"
)

func TestResToChat_TranslateStream(t *testing.T) {
	// Simulate a Chat Completions SSE stream.
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

	tr := &ResToChat{}
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
}

func TestResToChat_TranslateRequest_PrependsInstructionsAsSystemMessage(t *testing.T) {
	tr := &ResToChat{}

	body := json.RawMessage(`{
		"model": "test-model",
		"instructions": "You are a summarizer. Produce a handoff summary.",
		"input": [
			{"type": "message", "role": "user", "content": "hello"}
		]
	}`)

	req := &Request{Body: body, Model: "test-model"}
	upstream, err := tr.TranslateRequest(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var chatReq chat.ChatCompletionRequest
	if err := json.Unmarshal(upstream.Body, &chatReq); err != nil {
		t.Fatalf("failed to unmarshal upstream body: %v", err)
	}

	if len(chatReq.Messages) < 2 {
		t.Fatalf("expected at least 2 messages (system + user), got %d", len(chatReq.Messages))
	}

	msg0 := chatReq.Messages[0]
	if msg0.Role != "system" {
		t.Errorf("expected first message role 'system', got %q", msg0.Role)
	}
	if msg0.Content == nil || msg0.Content.String == nil {
		t.Fatal("expected first message to have string content")
	}
	if *msg0.Content.String != "You are a summarizer. Produce a handoff summary." {
		t.Errorf("expected instructions text, got %q", *msg0.Content.String)
	}
}

func TestResToChat_TranslateRequest_NoInstructions_NoSystemMessage(t *testing.T) {
	tr := &ResToChat{}

	body := json.RawMessage(`{
		"model": "test-model",
		"input": [
			{"type": "message", "role": "user", "content": "hello"}
		]
	}`)

	req := &Request{Body: body, Model: "test-model"}
	upstream, err := tr.TranslateRequest(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var chatReq chat.ChatCompletionRequest
	if err := json.Unmarshal(upstream.Body, &chatReq); err != nil {
		t.Fatalf("failed to unmarshal upstream body: %v", err)
	}

	for _, msg := range chatReq.Messages {
		if msg.Role == "system" {
			t.Error("expected no system message when instructions is absent")
		}
	}
}

func TestResToChat_TranslateRequest_IgnoresSessionMessages(t *testing.T) {
	tr := &ResToChat{}

	// Session has stale messages from a previous turn.
	s := &session.Session{
		
	}

	// Request body has different input items — this is the true source.
	body := json.RawMessage(`{
		"model": "test-model",
		"input": [
			{"type": "message", "role": "user", "content": "fresh request message"}
		]
	}`)

	req := &Request{Body: body, Model: "test-model"}
	upstream, err := tr.TranslateRequest(context.Background(), req, s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var chatReq chat.ChatCompletionRequest
	if err := json.Unmarshal(upstream.Body, &chatReq); err != nil {
		t.Fatalf("failed to unmarshal upstream body: %v", err)
	}

	if len(chatReq.Messages) != 1 {
		t.Fatalf("expected 1 message (from request body), got %d", len(chatReq.Messages))
	}
	if chatReq.Messages[0].Content == nil || chatReq.Messages[0].Content.String == nil {
		t.Fatal("expected message content")
	}
	if *chatReq.Messages[0].Content.String != "fresh request message" {
		t.Errorf("expected 'fresh request message', got %q", *chatReq.Messages[0].Content.String)
	}
}


func TestResToChat_TranslateStream_ReasoningContent(t *testing.T) {
	sseStream := strings.Join([]string{
		`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"delta":{"role":"assistant","reasoning_content":"Let me think about this."},"index":0}]}`,
		``,
		`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"delta":{"content":"Answer is 42"},"index":0}]}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	tr := &ResToChat{}
	ch := tr.TranslateStream(context.Background(), strings.NewReader(sseStream), nil, nil)

	var events []SSEEvent
	for ev := range ch {
		events = append(events, ev)
	}

	var reasoningDeltas []string
	for _, ev := range events {
		if ev.Event == "response.reasoning_summary_text.delta" {
			var delta struct {
				Delta string `json:"delta"`
			}
			if err := json.Unmarshal(ev.Data, &delta); err == nil {
				reasoningDeltas = append(reasoningDeltas, delta.Delta)
			}
		}
	}
	if len(reasoningDeltas) == 0 {
		t.Error("expected reasoning_summary_text.delta events, got none")
	}
	if len(reasoningDeltas) > 0 && reasoningDeltas[0] != "Let me think about this." {
		t.Errorf("expected reasoning delta 'Let me think about this.', got %q", reasoningDeltas[0])
	}
}
