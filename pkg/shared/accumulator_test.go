package shared

import (
	"testing"
)

func TestAccumulator_SingleDeltaNoFragmentation(t *testing.T) {
	acc := NewAccumulator()
	args, complete := acc.Feed(ToolCallDelta{
		CallID:    "call_1",
		Index:     0,
		Arguments: `{"name":"get_weather","arguments":{"city":"NYC"}}`,
		IsFinal:   true,
	})

	if !complete {
		t.Error("expected complete=true for single delta")
	}
	if args != `{"name":"get_weather","arguments":{"city":"NYC"}}` {
		t.Errorf("unexpected args: %s", args)
	}
}

func TestAccumulator_MultiDeltaAccumulation(t *testing.T) {
	acc := NewAccumulator()

	args1, complete1 := acc.Feed(ToolCallDelta{
		CallID: "call_1", Index: 0, Arguments: `{"name":"get_`,
	})
	if complete1 {
		t.Error("expected complete=false after first delta")
	}
	if args1 != `{"name":"get_` {
		t.Errorf("unexpected args1: %s", args1)
	}

	_, complete2 := acc.Feed(ToolCallDelta{
		CallID: "call_1", Index: 0, Arguments: `weather","arguments`,
	})
	if complete2 {
		t.Error("expected complete=false after second delta")
	}

	args3, complete3 := acc.Feed(ToolCallDelta{
		CallID: "call_1", Index: 0, Arguments: `":{"city":"NYC"}}`, IsFinal: true,
	})
	if !complete3 {
		t.Error("expected complete=true after final delta")
	}
	if args3 != `{"name":"get_weather","arguments":{"city":"NYC"}}` {
		t.Errorf("unexpected final args: %s", args3)
	}
}

func TestAccumulator_InterleavedToolCalls(t *testing.T) {
	acc := NewAccumulator()

	// Tool call A: first chunk
	acc.Feed(ToolCallDelta{
		CallID: "call_a", Index: 0, Arguments: `{"city":"`,
	})
	// Tool call B: first chunk
	acc.Feed(ToolCallDelta{
		CallID: "call_b", Index: 1, Arguments: `{"query":"`,
	})
	// Tool call A: second chunk
	acc.Feed(ToolCallDelta{
		CallID: "call_a", Index: 0, Arguments: `NYC"}`,
	})
	// Tool call B: second chunk
	acc.Feed(ToolCallDelta{
		CallID: "call_b", Index: 1, Arguments: `weather"}`,
	})

	if acc.Get("call_a") != `{"city":"NYC"}` {
		t.Errorf("call_a: expected %q, got %q", `{"city":"NYC"}`, acc.Get("call_a"))
	}
	if acc.Get("call_b") != `{"query":"weather"}` {
		t.Errorf("call_b: expected %q, got %q", `{"query":"weather"}`, acc.Get("call_b"))
	}
}

func TestAccumulator_EmptyInitialDelta(t *testing.T) {
	acc := NewAccumulator()

	// First delta is empty (common when tool call is first announced)
	args1, complete1 := acc.Feed(ToolCallDelta{
		CallID: "call_1", Index: 0, Arguments: "",
	})
	if complete1 {
		t.Error("expected complete=false for empty initial delta")
	}
	if args1 != "" {
		t.Errorf("expected empty args, got %q", args1)
	}

	// Second delta has actual content
	args2, complete2 := acc.Feed(ToolCallDelta{
		CallID: "call_1", Index: 0, Arguments: `{"name":"search"}`,
		IsFinal: true,
	})
	if !complete2 {
		t.Error("expected complete=true for final delta")
	}
	if args2 != `{"name":"search"}` {
		t.Errorf("unexpected final args: %s", args2)
	}
}

func TestAccumulator_RapidSuccessiveChunks(t *testing.T) {
	acc := NewAccumulator()
	chunks := []string{"{", `"n`, `ame`, `":`, `"s`, `earch`, `","`, `arg`, `s":`, `{`, `"q`, `uery`, `":`, `"he`, `llo`, `"}`, `}`}

	for i, chunk := range chunks {
		isFinal := i == len(chunks)-1
		_, complete := acc.Feed(ToolCallDelta{
			CallID: "call_1", Index: 0, Arguments: chunk, IsFinal: isFinal,
		})
		if isFinal && !complete {
			t.Errorf("chunk %d: expected complete=true for final chunk", i)
		}
		if !isFinal && complete {
			t.Errorf("chunk %d: expected complete=false for non-final chunk", i)
		}
	}

	expected := `{"name":"search","args":{"query":"hello"}}`
	if acc.Get("call_1") != expected {
		t.Errorf("expected %q, got %q", expected, acc.Get("call_1"))
	}
}
