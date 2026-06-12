package shared

import (
	"bufio"
	"io"
	"strings"
)

// SSEEvent represents a parsed Server-Sent Events event.
type SSEEvent struct {
	Event string // event type, e.g. "message", "ping"
	Data  string // accumulated data content
	ID    string // event ID
}

// hasContent reports whether the event has any field set.
func (e *SSEEvent) hasContent() bool {
	return e.Data != "" || e.Event != "" || e.ID != ""
}

// ParseSSE reads from r line-by-line, emitting parsed SSE events on a channel.
// The channel is closed when the reader is exhausted or an error occurs.
// Handles single-line data, multi-line data, event: and id: lines,
// empty-line delimiters, and the [DONE] sentinel.
func ParseSSE(r io.Reader) <-chan SSEEvent {
	ch := make(chan SSEEvent)
	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(r)
		var current SSEEvent
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				if current.hasContent() {
					ch <- current
					current = SSEEvent{}
				}
				continue
			}
			if after, ok := strings.CutPrefix(line, "data:"); ok {
				data := strings.TrimSpace(after)
				if current.Data != "" {
					current.Data += "\n"
				}
				current.Data += data
			} else if after, ok := strings.CutPrefix(line, "event:"); ok {
				current.Event = strings.TrimSpace(after)
			} else if after, ok := strings.CutPrefix(line, "id:"); ok {
				current.ID = strings.TrimSpace(after)
			}
		}
		// Emit any remaining event
		if current.hasContent() {
			ch <- current
		}
	}()
	return ch
}
