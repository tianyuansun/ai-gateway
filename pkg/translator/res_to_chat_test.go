package translator

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
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
