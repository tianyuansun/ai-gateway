package translator

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/tianyuansun/ai-gateway/pkg/schema/chat"
	"github.com/tianyuansun/ai-gateway/pkg/session"
)

func TestResToChat_UpdateSessionNoEmptyReasoning(t *testing.T) {
	tr := &ResToChat{}
	s := &session.Session{ID: "sess-1"}
	rawJSON := `{"id":"c1","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"simple answer"}}]}`
	httpResp := &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader([]byte(rawJSON))), Header: http.Header{"Content-Type": []string{"application/json"}}}
	req := &Request{Model: "gpt-4o", APIFormat: FormatResponses}
	resp, err := tr.TranslateResponse(context.Background(), httpResp, req, s)
	if err != nil { t.Fatalf("TranslateResponse: %v", err) }
	tr.UpdateSession(s, req, resp)
	if len(s.ReasoningRecords) != 0 {
		t.Errorf("expected 0 reasoning records, got %d", len(s.ReasoningRecords))
	}
}

func TestResToChat_UpdateSessionStoresReasoning(t *testing.T) {
	tr := &ResToChat{}
	s := &session.Session{ID: "sess-1", Messages: []session.Message{{Role: "user", Content: "think step by step"}}}
	rawJSON := `{"id":"c1","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"The answer is 4.","reasoning_content":"Let me count... 2+2=4"}}]}`
	httpResp := &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader([]byte(rawJSON))), Header: http.Header{"Content-Type": []string{"application/json"}}}
	req := &Request{Model: "deepseek-r1", APIFormat: FormatResponses}
	resp, err := tr.TranslateResponse(context.Background(), httpResp, req, s)
	if err != nil { t.Fatalf("TranslateResponse: %v", err) }
	tr.UpdateSession(s, req, resp)
	if len(s.ReasoningRecords) == 0 { t.Fatal("expected reasoning records") }
	if s.ReasoningRecords[0].Content != "Let me count... 2+2=4" {
		t.Errorf("expected reasoning, got %q", s.ReasoningRecords[0].Content)
	}
}

func TestResToChat_TranslateRequestInjectsReasoning(t *testing.T) {
	tr := &ResToChat{}
	s := &session.Session{
		ID: "sess-1",
		Messages: []session.Message{
			{Role: "user", Content: "think step by step"},
			{Role: "assistant", Content: "The answer is 4."},
			{Role: "user", Content: "continue"},
		},
		ReasoningRecords: []session.Reasoning{{Content: "Previous step-by-step reasoning..."}},
	}
	body := responsesRequest(t, "deepseek-r1", "continue")
	req := &Request{Model: "deepseek-r1", APIFormat: FormatResponses, Body: body}
	upReq, err := tr.TranslateRequest(context.Background(), req, s)
	if err != nil { t.Fatalf("TranslateRequest: %v", err) }
	var chatReq chat.ChatCompletionRequest
	if err := json.Unmarshal(upReq.Body, &chatReq); err != nil { t.Fatalf("unmarshal: %v", err) }
	found := false
	for _, msg := range chatReq.Messages {
		if msg.ReasoningContent != "" { found = true; break }
	}
	if !found { t.Error("expected reasoning_content in messages") }
}
