package translator

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestChatToAnth_TranslateStream(t *testing.T) {
	// Anthropic SSE → Chat Completions SSE.
	sseStream := strings.Join([]string{
		`event: message_start`,
		`data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude","usage":{"input_tokens":5}}}`,
		``,
		`event: content_block_start`,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hey"}}`,
		``,
		`event: content_block_stop`,
		`data: {"type":"content_block_stop","index":0}`,
		``,
		`event: message_stop`,
		`data: {"type":"message_stop"}`,
		``,
	}, "\n")

	tr := &ChatToAnth{}
	ch := tr.TranslateStream(context.Background(), strings.NewReader(sseStream), nil, nil)

	var events []SSEEvent
	for ev := range ch {
		events = append(events, ev)
	}

	if len(events) < 1 {
		t.Fatalf("expected at least 1 event, got %d", len(events))
	}

	// Accumulate text from delta content.
	var allText string
	for _, ev := range events {
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(ev.Data, &chunk); err == nil {
			for _, choice := range chunk.Choices {
				allText += choice.Delta.Content
			}
		}
	}

	if allText != "Hey" {
		t.Errorf("expected accumulated text 'Hey', got %q", allText)
	}
}
