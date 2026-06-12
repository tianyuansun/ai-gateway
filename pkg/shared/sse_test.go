package shared

import (
	"strings"
	"testing"
)

func TestParseSSE_SingleLineData(t *testing.T) {
	input := "data: {\"key\":\"val\"}\n\n"
	ch := ParseSSE(strings.NewReader(input))

	var events []SSEEvent
	for ev := range ch {
		events = append(events, ev)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Data != `{"key":"val"}` {
		t.Errorf("expected Data = %q, got %q", `{"key":"val"}`, events[0].Data)
	}
}

func TestParseSSE_MultiLineData(t *testing.T) {
	input := "data: line1\ndata: line2\ndata: line3\n\n"
	ch := ParseSSE(strings.NewReader(input))

	var events []SSEEvent
	for ev := range ch {
		events = append(events, ev)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Data != "line1\nline2\nline3" {
		t.Errorf("expected Data = %q, got %q", "line1\nline2\nline3", events[0].Data)
	}
}

func TestParseSSE_EventAndIDLines(t *testing.T) {
	input := "event: update\nid: 42\ndata: {\"x\":1}\n\n"
	ch := ParseSSE(strings.NewReader(input))

	var events []SSEEvent
	for ev := range ch {
		events = append(events, ev)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Event != "update" {
		t.Errorf("expected Event = %q, got %q", "update", events[0].Event)
	}
	if events[0].ID != "42" {
		t.Errorf("expected ID = %q, got %q", "42", events[0].ID)
	}
	if events[0].Data != `{"x":1}` {
		t.Errorf("expected Data = %q, got %q", `{"x":1}`, events[0].Data)
	}
}

func TestParseSSE_EmptyLineDelimitsEvents(t *testing.T) {
	input := "data: first\n\ndata: second\n\ndata: third\n\n"
	ch := ParseSSE(strings.NewReader(input))

	var events []SSEEvent
	for ev := range ch {
		events = append(events, ev)
	}

	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	if events[0].Data != "first" {
		t.Errorf("event 0: expected Data = %q, got %q", "first", events[0].Data)
	}
	if events[1].Data != "second" {
		t.Errorf("event 1: expected Data = %q, got %q", "second", events[1].Data)
	}
	if events[2].Data != "third" {
		t.Errorf("event 2: expected Data = %q, got %q", "third", events[2].Data)
	}
}

func TestParseSSE_DoneSentinel(t *testing.T) {
	input := "data: [DONE]\n\n"
	ch := ParseSSE(strings.NewReader(input))

	var events []SSEEvent
	for ev := range ch {
		events = append(events, ev)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Data != "[DONE]" {
		t.Errorf("expected Data = %q, got %q", "[DONE]", events[0].Data)
	}
}
