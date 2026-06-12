package translator

import (
	"context"
	"strings"
	"testing"
)

func TestPassthroughTranslator_TranslateStream(t *testing.T) {
	// Build a realistic Cham Completions SSE stream.
	sseStream := strings.Join([]string{
		`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"delta":{"role":"assistant"},"index":0}]}`,
		``,
		`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"delta":{"content":"Hello"},"index":0}]}`,
		``,
		`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"delta":{"content":" world"},"index":0}]}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	pt := &PassthroughTranslator{}
	ch := pt.TranslateStream(context.Background(), strings.NewReader(sseStream), nil, nil)

	var events []SSEEvent
	for ev := range ch {
		events = append(events, ev)
	}

	if len(events) < 3 {
		t.Fatalf("expected at least 3 events, got %d", len(events))
	}

	// Second event should contain "Hello"
	if !strings.Contains(string(events[1].Data), "Hello") {
		t.Errorf("event 1: expected data containing 'Hello', got %s", string(events[1].Data))
	}

	// Third event should contain " world"
	if !strings.Contains(string(events[2].Data), "world") {
		t.Errorf("event 2: expected data containing ' world', got %s", string(events[2].Data))
	}
}
