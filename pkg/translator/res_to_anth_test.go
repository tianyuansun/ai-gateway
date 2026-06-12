package translator

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/tianyuansun/ai-gateway/pkg/schema/anthropic"
	"github.com/tianyuansun/ai-gateway/pkg/schema/responses"
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

	var anthReq anthropic.MessageRequest
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

func TestResToAnth_TranslateResponseReadsHTTPBody(t *testing.T) {
	anthResp := anthropic.MessageResponse{
		ID:   "msg_1",
		Type: "message",
		Role: "assistant",
		Content: []anthropic.ResponseContentBlock{
			{Type: "text", Text: "hello from anthropic"},
		},
		Usage: anthropic.Usage{InputTokens: 10, OutputTokens: 5},
	}
	respBody, _ := json.Marshal(anthResp)

	httpResp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader(respBody)),
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"X-Request-Id": []string{"req-123"},
		},
	}

	tr := &ResToAnth{}
	req := &Request{
		Model:     "ds-pro",
		APIFormat: FormatResponses,
	}

	result, err := tr.TranslateResponse(context.Background(), httpResp, req, nil)
	if err != nil {
		t.Fatalf("TranslateResponse: %v", err)
	}
	if result.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", result.StatusCode)
	}
	if result.Body == nil {
		t.Fatal("expected non-nil body")
	}

	var responsesResp responses.Response
	if err := json.Unmarshal(result.Body, &responsesResp); err != nil {
		t.Fatalf("unmarshal result body: %v", err)
	}
	if responsesResp.ID != "msg_1" {
		t.Errorf("expected ID msg_1, got %s", responsesResp.ID)
	}
	if responsesResp.Usage == nil || responsesResp.Usage.TotalTokens != 15 {
		t.Errorf("expected total tokens 15, got %+v", responsesResp.Usage)
	}
}
