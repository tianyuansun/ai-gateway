package translator

import (
	"encoding/json"
	"testing"

	"github.com/tianyuansun/ai-gateway/pkg/schema/anthropic"
	"github.com/tianyuansun/ai-gateway/pkg/schema/responses"
)

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

	var anthReq anthropic.MessageRequest
	if err := json.Unmarshal(upReq.Body, &anthReq); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if len(anthReq.Messages) != 1 {
		t.Fatalf("expected 1 message from body, got %d", len(anthReq.Messages))
	}
}

func responsesRequest(t *testing.T, model, text string) []byte {
	t.Helper()
	textJSON, _ := json.Marshal(text)
	b, _ := json.Marshal(responses.ResponseRequest{
		Model: model,
		Input: responses.ResponseInput{
			Items: []responses.ResponseInputItem{
				{Type: "message", Role: "user", Content: json.RawMessage(textJSON)},
			},
		},
	})
	return b
}
