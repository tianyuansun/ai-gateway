package translator

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestResToAnth_TranslateStream(t *testing.T) {
	// Simulate an Anthropic Messages SSE stream.
	sseStream := strings.Join([]string{
		`event: message_start`,
		`data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-4-6","usage":{"input_tokens":10}}}`,
		``,
		`event: content_block_start`,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}`,
		``,
		`event: content_block_stop`,
		`data: {"type":"content_block_stop","index":0}`,
		``,
		`event: message_delta`,
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":15}}`,
		``,
		`event: message_stop`,
		`data: {"type":"message_stop"}`,
		``,
	}, "\n")

	tr := &ResToAnth{}
	ch := tr.TranslateStream(context.Background(), strings.NewReader(sseStream), nil, nil)

	var events []SSEEvent
	for ev := range ch {
		events = append(events, ev)
	}

	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}

	// First event should indicate response start.
	if events[0].Event != "response.created" {
		t.Errorf("event 0: expected 'response.created', got %q", events[0].Event)
	}

	// Accumulate text from delta events.
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
