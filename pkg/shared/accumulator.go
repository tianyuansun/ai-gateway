package shared

// ToolCallDelta represents a single delta update for a tool call's arguments.
type ToolCallDelta struct {
	CallID    string // unique identifier for the tool call
	Index     int    // position of this tool call in the response
	Arguments string // partial JSON fragment (may be empty)
	IsFinal   bool   // true if this is the last delta for this call
}

// Accumulator buffers fragmented tool-call argument deltas and reconstructs
// complete argument strings per call_id.
type Accumulator struct {
	buf map[string]string
}

// NewAccumulator creates a new Accumulator.
func NewAccumulator() *Accumulator {
	return &Accumulator{
		buf: make(map[string]string),
	}
}

// Feed processes a delta. It returns the accumulated arguments so far and
// whether the tool-call block is complete (true when no more fragments expected).
// For Chat Completions SSE, a tool call is complete when the delta has content
// and the overall streaming response indicates the tool call block ended.
// Callers should treat !isComplete as "accumulate more", isComplete as "ready to use".
func (a *Accumulator) Feed(d ToolCallDelta) (args string, isComplete bool) {
	key := d.CallID
	current := a.buf[key]
	current += d.Arguments
	a.buf[key] = current
	return current, d.IsFinal
}

// Get returns the current accumulated arguments for a call_id.
func (a *Accumulator) Get(callID string) string {
	return a.buf[callID]
}
