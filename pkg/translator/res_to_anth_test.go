package translator

import (
	"encoding/json"
	"testing"

	"github.com/tianyuansun/ai-gateway/pkg/session"
)

func TestResToAnth_UsesSessionMessages(t *testing.T) {
	s := &session.Session{
		Messages: []session.Message{
			{Role: "user", Content: "what is 2+2"},
			{Role: "assistant", Content: "4"},
			{Role: "user", Content: "now multiply by 3"},
		},
	}

	tr := &ResToAnth{}
	// Body is a new turn — it should be ignored when session has messages.
	body := responsesRequest(t, "ds-pro", "what is the answer?")
	req := &Request{
		Model:     "ds-pro",
		APIFormat: FormatResponses,
		Body:      body,
	}

	upReq, err := tr.TranslateRequest(nil, req, s)
	if err != nil {
		t.Fatalf("TranslateRequest: %v", err)
	}

	var anthReq AnthropicRequest
	if err := json.Unmarshal(upReq.Body, &anthReq); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if len(anthReq.Messages) != 3 {
		t.Fatalf("expected 3 messages from session, got %d", len(anthReq.Messages))
	}
	if anthReq.Messages[0].Role != "user" || anthReq.Messages[0].Content[0].Text != "what is 2+2" {
		t.Errorf("first message mismatch: %+v", anthReq.Messages[0])
	}
	if anthReq.Messages[1].Role != "assistant" || anthReq.Messages[1].Content[0].Text != "4" {
		t.Errorf("second message mismatch: %+v", anthReq.Messages[1])
	}
}

func TestResToAnth_NilSessionBuildsFromBody(t *testing.T) {
	tr := &ResToAnth{}
	body := responsesRequest(t, "ds-pro", "hello")
	req := &Request{
		Model:     "ds-pro",
		APIFormat: FormatResponses,
		Body:      body,
	}

	upReq, err := tr.TranslateRequest(nil, req, nil)
	if err != nil {
		t.Fatalf("TranslateRequest: %v", err)
	}

	var anthReq AnthropicRequest
	if err := json.Unmarshal(upReq.Body, &anthReq); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if len(anthReq.Messages) != 1 {
		t.Fatalf("expected 1 message from body, got %d", len(anthReq.Messages))
	}
}

func responsesRequest(t *testing.T, model, text string) []byte {
	t.Helper()
	b, _ := json.Marshal(ResponsesRequest{
		Model: model,
		Input: []InputItem{
			{Type: "message", Role: "user", Content: []ContentPart{{Type: "input_text", Text: text}}},
		},
	})
	return b
}
