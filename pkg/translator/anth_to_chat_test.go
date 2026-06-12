package translator

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestAnthToChat_TranslateStream(t *testing.T) {
	// Chat Completions SSE → Anthropic SSE.
	sseStream := strings.Join([]string{
		`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"delta":{"role":"assistant"},"index":0}]}`,
		``,
		`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"delta":{"content":"Hi"},"index":0}]}`,
		``,
		`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"delta":{},"finish_reason":"stop","index":0}]}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	tr := &AnthToChat{}
	ch := tr.TranslateStream(context.Background(), strings.NewReader(sseStream), nil, nil)

	var events []SSEEvent
	for ev := range ch {
		events = append(events, ev)
	}

	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}

	// First event should be message_start.
	if events[0].Event != "message_start" {
		t.Errorf("event 0: expected 'message_start', got %q", events[0].Event)
	}

	// Find content_block_delta events with text.
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
			if err := json.Unmarshal(ev.Data, &delta); err == nil {
				if delta.Delta.Type == "text_delta" {
					allText += delta.Delta.Text
				}
			}
		}
	}
	if allText != "Hi" {
		t.Errorf("expected accumulated text 'Hi', got %q", allText)
	}

	// Last event should be message_stop.
	last := events[len(events)-1]
	if last.Event != "message_stop" {
		t.Errorf("last event: expected 'message_stop', got %q", last.Event)
	}
}
