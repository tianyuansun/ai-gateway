package translator

import (
	"context"
	"strings"
	"testing"

	"github.com/tianyuansun/ai-gateway/pkg/session"
)

// =============================================================================
// ResToAnth edge-case tests
// =============================================================================

func TestResToAnth_TranslateRequest_EmptyBody(t *testing.T) {
	tr := &ResToAnth{}
	req := &Request{
		Model:     "ds-pro",
		APIFormat: FormatResponses,
		Body:      []byte{},
	}

	_, err := tr.TranslateRequest(context.Background(), req, nil)
	if err == nil {
		t.Fatal("expected error for empty request body, got nil")
	}
}

func TestResToAnth_TranslateRequest_MalformedJSON(t *testing.T) {
	tr := &ResToAnth{}
	req := &Request{
		Model:     "ds-pro",
		APIFormat: FormatResponses,
		Body:      []byte("not-json"),
	}

	_, err := tr.TranslateRequest(context.Background(), req, nil)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestResToAnth_TranslateRequest_MissingModel(t *testing.T) {
	tr := &ResToAnth{}
	// Body has no "model" field — body.Model will be empty, but req.Model is used instead.
	body := `{"input": [{"type": "message", "role": "user", "content": "hello"}]}`
	req := &Request{
		Model:     "ds-pro",
		APIFormat: FormatResponses,
		Body:      []byte(body),
	}

	upReq, err := tr.TranslateRequest(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("TranslateRequest should succeed with missing model in body: %v", err)
	}
	if upReq == nil {
		t.Fatal("expected non-nil upstream request")
	}
}

// TestResToAnth_NilSessionBuildsFromBody is already covered in res_to_anth_test.go.

func TestResToAnth_TranslateRequest_EmptyInputItems(t *testing.T) {
	tr := &ResToAnth{}
	body := `{"model": "ds-pro", "input": []}` // empty input items
	req := &Request{
		Model:     "ds-pro",
		APIFormat: FormatResponses,
		Body:      []byte(body),
	}

	upReq, err := tr.TranslateRequest(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("TranslateRequest should not error on empty input: %v", err)
	}
	// With empty input items and nil session, messages should be empty.
	if !strings.Contains(string(upReq.Body), `"messages":[]`) &&
		!strings.Contains(string(upReq.Body), `"messages":null`) {
		t.Logf("upstream body for empty input: %s", string(upReq.Body))
	}
}

func TestResToAnth_TranslateStream_EmptyUpstream(t *testing.T) {
	tr := &ResToAnth{}
	ch := tr.TranslateStream(context.Background(), strings.NewReader(""), nil, nil)

	var events []SSEEvent
	for ev := range ch {
		events = append(events, ev)
	}

	if len(events) != 0 {
		t.Errorf("expected 0 events from empty upstream, got %d", len(events))
	}
}

// =============================================================================
// ResToChat edge-case tests
// =============================================================================

func TestResToChat_TranslateRequest_EmptyBody(t *testing.T) {
	tr := &ResToChat{}
	req := &Request{
		Model:     "gpt-4",
		APIFormat: FormatResponses,
		Body:      []byte{},
	}

	_, err := tr.TranslateRequest(context.Background(), req, nil)
	if err == nil {
		t.Fatal("expected error for empty request body, got nil")
	}
}

func TestResToChat_TranslateRequest_MalformedJSON(t *testing.T) {
	tr := &ResToChat{}
	req := &Request{
		Model:     "gpt-4",
		APIFormat: FormatResponses,
		Body:      []byte("garbage"),
	}

	_, err := tr.TranslateRequest(context.Background(), req, nil)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestResToChat_TranslateRequest_MissingModel(t *testing.T) {
	tr := &ResToChat{}
	body := `{"input": [{"type": "message", "role": "user", "content": "hello"}]}`
	req := &Request{
		Model:     "gpt-4",
		APIFormat: FormatResponses,
		Body:      []byte(body),
	}

	upReq, err := tr.TranslateRequest(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("TranslateRequest should succeed with missing model in body: %v", err)
	}
	if upReq == nil {
		t.Fatal("expected non-nil upstream request")
	}
}

func TestResToChat_TranslateRequest_NilSession(t *testing.T) {
	tr := &ResToChat{}
	body := `{"model": "gpt-4", "input": [{"type": "message", "role": "user", "content": "hello"}]}`
	req := &Request{
		Model:     "gpt-4",
		APIFormat: FormatResponses,
		Body:      []byte(body),
	}

	upReq, err := tr.TranslateRequest(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("TranslateRequest with nil session should not error: %v", err)
	}
	if upReq == nil {
		t.Fatal("expected non-nil upstream request")
	}
	if !strings.Contains(string(upReq.Body), "hello") {
		t.Errorf("upstream body should contain 'hello', got: %s", string(upReq.Body))
	}
}

func TestResToChat_TranslateRequest_EmptyInputItems(t *testing.T) {
	tr := &ResToChat{}
	body := `{"model": "gpt-4", "input": []}`
	req := &Request{
		Model:     "gpt-4",
		APIFormat: FormatResponses,
		Body:      []byte(body),
	}

	upReq, err := tr.TranslateRequest(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("TranslateRequest should not error on empty input: %v", err)
	}
	if !strings.Contains(string(upReq.Body), `"messages":[]`) &&
		!strings.Contains(string(upReq.Body), `"messages":null`) {
		t.Logf("upstream body for empty input: %s", string(upReq.Body))
	}
}

func TestResToChat_TranslateStream_EmptyUpstream(t *testing.T) {
	tr := &ResToChat{}
	ch := tr.TranslateStream(context.Background(), strings.NewReader(""), nil, nil)

	var events []SSEEvent
	for ev := range ch {
		events = append(events, ev)
	}

	if len(events) != 0 {
		t.Errorf("expected 0 events from empty upstream, got %d", len(events))
	}
}

// =============================================================================
// AnthToChat edge-case tests
// =============================================================================

func validAnthropicBody() []byte {
	return []byte(`{"model": "claude-3", "messages": [{"role": "user", "content": [{"type": "text", "text": "hello"}]}], "max_tokens": 100}`)
}

func TestAnthToChat_TranslateRequest_EmptyBody(t *testing.T) {
	tr := &AnthToChat{}
	req := &Request{
		Model:     "claude-3",
		APIFormat: FormatAnthropic,
		Body:      []byte{},
	}

	_, err := tr.TranslateRequest(context.Background(), req, nil)
	if err == nil {
		t.Fatal("expected error for empty request body, got nil")
	}
}

func TestAnthToChat_TranslateRequest_MalformedJSON(t *testing.T) {
	tr := &AnthToChat{}
	req := &Request{
		Model:     "claude-3",
		APIFormat: FormatAnthropic,
		Body:      []byte("bad json"),
	}

	_, err := tr.TranslateRequest(context.Background(), req, nil)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestAnthToChat_TranslateRequest_MissingModel(t *testing.T) {
	tr := &AnthToChat{}
	// Anthropic body without model field
	body := `{"messages": [{"role": "user", "content": [{"type": "text", "text": "hello"}]}], "max_tokens": 100}`
	req := &Request{
		Model:     "claude-3",
		APIFormat: FormatAnthropic,
		Body:      []byte(body),
	}

	upReq, err := tr.TranslateRequest(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("TranslateRequest should succeed with missing model in body: %v", err)
	}
	if upReq == nil {
		t.Fatal("expected non-nil upstream request")
	}
}

func TestAnthToChat_TranslateRequest_NilSession(t *testing.T) {
	tr := &AnthToChat{}
	req := &Request{
		Model:     "claude-3",
		APIFormat: FormatAnthropic,
		Body:      validAnthropicBody(),
	}

	upReq, err := tr.TranslateRequest(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("TranslateRequest with nil session should not error: %v", err)
	}
	if upReq == nil {
		t.Fatal("expected non-nil upstream request")
	}
	if !strings.Contains(string(upReq.Body), "hello") {
		t.Errorf("upstream body should contain 'hello', got: %s", string(upReq.Body))
	}
}

func TestAnthToChat_TranslateRequest_EmptyInputItems(t *testing.T) {
	tr := &AnthToChat{}
	body := `{"model": "claude-3", "messages": [], "max_tokens": 100}`
	req := &Request{
		Model:     "claude-3",
		APIFormat: FormatAnthropic,
		Body:      []byte(body),
	}

	upReq, err := tr.TranslateRequest(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("TranslateRequest should not error on empty messages: %v", err)
	}
	if !strings.Contains(string(upReq.Body), `"messages":[]`) {
		t.Logf("upstream body for empty messages: %s", string(upReq.Body))
	}
}

func TestAnthToChat_TranslateStream_EmptyUpstream(t *testing.T) {
	tr := &AnthToChat{}
	ch := tr.TranslateStream(context.Background(), strings.NewReader(""), nil, nil)

	var events []SSEEvent
	for ev := range ch {
		events = append(events, ev)
	}

	if len(events) != 0 {
		t.Errorf("expected 0 events from empty upstream, got %d", len(events))
	}
}

// =============================================================================
// ChatToAnth edge-case tests
// =============================================================================

func validChatBody() []byte {
	return []byte(`{"model": "gpt-4", "messages": [{"role": "user", "content": "hello"}]}`)
}

func TestChatToAnth_TranslateRequest_EmptyBody(t *testing.T) {
	tr := &ChatToAnth{}
	req := &Request{
		Model:     "gpt-4",
		APIFormat: FormatChat,
		Body:      []byte{},
	}

	_, err := tr.TranslateRequest(context.Background(), req, nil)
	if err == nil {
		t.Fatal("expected error for empty request body, got nil")
	}
}

func TestChatToAnth_TranslateRequest_MalformedJSON(t *testing.T) {
	tr := &ChatToAnth{}
	req := &Request{
		Model:     "gpt-4",
		APIFormat: FormatChat,
		Body:      []byte("{{{"),
	}

	_, err := tr.TranslateRequest(context.Background(), req, nil)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestChatToAnth_TranslateRequest_MissingModel(t *testing.T) {
	tr := &ChatToAnth{}
	// Chat body without model field
	body := `{"messages": [{"role": "user", "content": "hello"}]}`
	req := &Request{
		Model:     "gpt-4",
		APIFormat: FormatChat,
		Body:      []byte(body),
	}

	upReq, err := tr.TranslateRequest(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("TranslateRequest should succeed with missing model in body: %v", err)
	}
	if upReq == nil {
		t.Fatal("expected non-nil upstream request")
	}
}

func TestChatToAnth_TranslateRequest_NilSession(t *testing.T) {
	tr := &ChatToAnth{}
	req := &Request{
		Model:     "gpt-4",
		APIFormat: FormatChat,
		Body:      validChatBody(),
	}

	upReq, err := tr.TranslateRequest(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("TranslateRequest with nil session should not error: %v", err)
	}
	if upReq == nil {
		t.Fatal("expected non-nil upstream request")
	}
	if !strings.Contains(string(upReq.Body), "hello") {
		t.Errorf("upstream body should contain 'hello', got: %s", string(upReq.Body))
	}
}

func TestChatToAnth_TranslateRequest_EmptyInputItems(t *testing.T) {
	tr := &ChatToAnth{}
	body := `{"model": "gpt-4", "messages": []}`
	req := &Request{
		Model:     "gpt-4",
		APIFormat: FormatChat,
		Body:      []byte(body),
	}

	upReq, err := tr.TranslateRequest(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("TranslateRequest should not error on empty messages: %v", err)
	}
	if !strings.Contains(string(upReq.Body), `"messages":[]`) &&
		!strings.Contains(string(upReq.Body), `"messages":null`) {
		t.Logf("upstream body for empty messages: %s", string(upReq.Body))
	}
}

func TestChatToAnth_TranslateStream_EmptyUpstream(t *testing.T) {
	tr := &ChatToAnth{}
	ch := tr.TranslateStream(context.Background(), strings.NewReader(""), nil, nil)

	var events []SSEEvent
	for ev := range ch {
		events = append(events, ev)
	}

	if len(events) != 0 {
		t.Errorf("expected 0 events from empty upstream, got %d", len(events))
	}
}

// =============================================================================
// PassthroughTranslator edge-case tests
// =============================================================================

func TestPassthroughTranslator_TranslateRequest_EmptyBody(t *testing.T) {
	pt := &PassthroughTranslator{}
	req := &Request{
		Model:     "gpt-4",
		APIFormat: FormatChat,
		Body:      []byte{},
	}

	upReq, err := pt.TranslateRequest(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("passthrough TranslateRequest should not error on empty body: %v", err)
	}
	if upReq == nil {
		t.Fatal("expected non-nil upstream request")
	}
	if len(upReq.Body) != 0 {
		t.Errorf("expected empty body in upstream request, got %d bytes", len(upReq.Body))
	}
}

func TestPassthroughTranslator_TranslateRequest_NilSession(t *testing.T) {
	pt := &PassthroughTranslator{}
	body := []byte(`{"model": "gpt-4", "messages": [{"role": "user", "content": "hello"}]}`)
	req := &Request{
		Model:     "gpt-4",
		APIFormat: FormatChat,
		Body:      body,
	}

	upReq, err := pt.TranslateRequest(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("passthrough TranslateRequest with nil session should not error: %v", err)
	}
	if upReq == nil {
		t.Fatal("expected non-nil upstream request")
	}
	if string(upReq.Body) != string(body) {
		t.Errorf("expected body to pass through unchanged")
	}
}

func TestPassthroughTranslator_TranslateStream_EmptyUpstream(t *testing.T) {
	pt := &PassthroughTranslator{}
	ch := pt.TranslateStream(context.Background(), strings.NewReader(""), nil, nil)

	var events []SSEEvent
	for ev := range ch {
		events = append(events, ev)
	}

	if len(events) != 0 {
		t.Errorf("expected 0 events from empty upstream, got %d", len(events))
	}
}

// =============================================================================
// Nil session edge case with session that has no messages (graceful fallback)
// =============================================================================

func TestResToChat_NilSession_EmptySessionMessages(t *testing.T) {
	// Session exists but has no messages — should fall back to body items.
	tr := &ResToChat{}
	s := &session.Session{
		Messages: []session.Message{}, // empty
	}
	body := `{"model": "gpt-4", "input": [{"type": "message", "role": "user", "content": "hello from body"}]}`
	req := &Request{
		Model:     "gpt-4",
		APIFormat: FormatResponses,
		Body:      []byte(body),
	}

	upReq, err := tr.TranslateRequest(context.Background(), req, s)
	if err != nil {
		t.Fatalf("TranslateRequest with empty session messages should not error: %v", err)
	}
	if !strings.Contains(string(upReq.Body), "hello from body") {
		t.Errorf("expected body input to be used when session messages are empty, got: %s", string(upReq.Body))
	}
}

func TestResToAnth_NilSession_EmptySessionMessages(t *testing.T) {
	// Session exists but has no messages — should fall back to body items.
	tr := &ResToAnth{}
	s := &session.Session{
		Messages: []session.Message{},
	}
	body := `{"model": "ds-pro", "input": [{"type": "message", "role": "user", "content": "hello from body"}]}`
	req := &Request{
		Model:     "ds-pro",
		APIFormat: FormatResponses,
		Body:      []byte(body),
	}

	upReq, err := tr.TranslateRequest(context.Background(), req, s)
	if err != nil {
		t.Fatalf("TranslateRequest with empty session messages should not error: %v", err)
	}
	if !strings.Contains(string(upReq.Body), "hello from body") {
		t.Errorf("expected body input to be used when session messages are empty, got: %s", string(upReq.Body))
	}
}
