package translator

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/tianyuansun/ai-gateway/pkg/schema/anthropic"
	"github.com/tianyuansun/ai-gateway/pkg/schema/responses"
	"github.com/tianyuansun/ai-gateway/pkg/session"
	"github.com/tianyuansun/ai-gateway/pkg/shared"
)

// AnthToRes translates Anthropic Messages API requests to OpenAI Responses API.
type AnthToRes struct{}

func (t *AnthToRes) TranslateRequest(_ context.Context, req *Request, _ *session.Session) (*UpstreamRequest, error) {
	var anthReq anthropic.MessageRequest
	if err := json.Unmarshal(req.Body, &anthReq); err != nil {
		return nil, err
	}

	resReq := t.buildResponseRequest(&anthReq, req.Model)
	resReq.Model = req.Model

	resBody, err := json.Marshal(resReq)
	if err != nil {
		return nil, err
	}

	return &UpstreamRequest{
		Method: "POST",
		URL:    "/responses",
		Body:   resBody,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}

func (t *AnthToRes) buildResponseRequest(anthReq *anthropic.MessageRequest, model string) *responses.ResponseRequest {
	resReq := &responses.ResponseRequest{
		Model: model,
	}

	// Map system to instructions.
	if anthReq.System != nil && anthReq.System.String != nil {
		resReq.Instructions = anthReq.System.String
	}

	// Map messages to input items.
	for _, msg := range anthReq.Messages {
		switch msg.Role {
		case "user":
			for _, c := range msg.Content {
				switch c.Type {
				case "text":
					textJSON, _ := json.Marshal(c.Text)
					resReq.Input.Items = append(resReq.Input.Items, responses.ResponseInputItem{
						Type:    "message",
						Role:    "user",
						Content: textJSON,
					})
				case "tool_result":
					resReq.Input.Items = append(resReq.Input.Items, responses.ResponseInputItem{
						Type:   "function_call_output",
						CallID: c.ToolUseID,
						Output: c.Content,
					})
				}
			}
		case "assistant":
			for _, c := range msg.Content {
				switch c.Type {
				case "text":
					textJSON, _ := json.Marshal(c.Text)
					resReq.Input.Items = append(resReq.Input.Items, responses.ResponseInputItem{
						Type:    "message",
						Role:    "assistant",
						Content: textJSON,
					})
				case "tool_use":
					resReq.Input.Items = append(resReq.Input.Items, responses.ResponseInputItem{
						Type:      "function_call",
						CallID:    c.ID,
						Name:      c.Name,
						Arguments: string(c.Input),
					})
				}
			}
		}
	}

	// Map tools.
	if len(anthReq.Tools) > 0 {
		resReq.Tools = make([]responses.ToolDefinition, 0, len(anthReq.Tools))
		for _, tool := range anthReq.Tools {
			resReq.Tools = append(resReq.Tools, responses.ToolDefinition{
				Type:        "function",
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			})
		}
	}

	// Map thinking to reasoning config.
	if anthReq.Thinking != nil && anthReq.Thinking.Type == "enabled" {
		effort := "high"
		summary := "auto"
		resReq.Reasoning = &responses.ReasoningConfig{
			Effort:  &effort,
			Summary: &summary,
		}
	}

	return resReq
}

func (t *AnthToRes) TranslateResponse(_ context.Context, upstream *http.Response, _ *Request, _ *session.Session) (*Response, error) {
	body, err := io.ReadAll(upstream.Body)
	if err != nil {
		return nil, err
	}
	upstream.Body.Close()

	var resResp responses.Response
	if err := json.Unmarshal(body, &resResp); err != nil {
		return nil, err
	}

	reasoningContent := t.extractReasoning(&resResp)
	anthResp := t.convertToAnthropicResponse(&resResp)

	anthBody, err := json.Marshal(anthResp)
	if err != nil {
		return nil, err
	}

	return &Response{
		StatusCode:       200,
		Body:             anthBody,
		ReasoningContent: reasoningContent,
	}, nil
}

func (t *AnthToRes) extractReasoning(resResp *responses.Response) string {
	var reasoning string
	for _, item := range resResp.Output {
		if item.Type == "reasoning" {
			for _, s := range item.Summary {
				var summary struct {
					Type string `json:"type"`
					Text string `json:"text"`
				}
				if json.Unmarshal(s, &summary) == nil && summary.Text != "" {
					reasoning += summary.Text
				}
			}
		}
	}
	return reasoning
}

func (t *AnthToRes) convertToAnthropicResponse(resResp *responses.Response) *anthropic.MessageResponse {
	content := []anthropic.ResponseContentBlock{}

	for _, item := range resResp.Output {
		switch item.Type {
		case "message":
			for _, part := range item.Content {
				if part.Type == "output_text" && part.Text != "" {
					content = append(content, anthropic.ResponseContentBlock{
						Type: "text",
						Text: part.Text,
					})
				}
			}
		case "function_call":
			content = append(content, anthropic.ResponseContentBlock{
				Type:  "tool_use",
				ID:    item.CallID,
				Name:  item.Name,
				Input: json.RawMessage(item.Arguments),
			})
		}
	}

	anthResp := &anthropic.MessageResponse{
		ID:      resResp.ID,
		Type:    "message",
		Role:    "assistant",
		Content: content,
	}

	if resResp.Usage != nil {
		anthResp.Usage = anthropic.Usage{
			InputTokens:  resResp.Usage.InputTokens,
			OutputTokens: resResp.Usage.OutputTokens,
		}
	}

	return anthResp
}

func (t *AnthToRes) TranslateStream(_ context.Context, upstream io.Reader, _ *Request, _ *session.Session) <-chan SSEEvent {
	ch := make(chan SSEEvent)
	go func() {
		defer close(ch)
		started := false
		completed := false
		itemStarted := false
		partStarted := false
		var msgID string
		for sseEv := range shared.ParseSSE(upstream) {
			var event struct {
				Type     string `json:"type"`
				Response struct {
					ID string `json:"id"`
				} `json:"response"`
				Item struct {
					ID   string `json:"id"`
					Type string `json:"type"`
					Role string `json:"role"`
				} `json:"item"`
				Part struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"part"`
				Delta string `json:"delta"`
			}
			if err := json.Unmarshal([]byte(sseEv.Data), &event); err != nil {
				continue
			}

			switch event.Type {
			case "response.created":
				if !started {
					msgID = event.Response.ID
					started = true
					startData, _ := json.Marshal(map[string]any{
						"type":    "message_start",
						"message": map[string]any{"id": msgID, "type": "message", "role": "assistant"},
					})
					ch <- SSEEvent{Event: "message_start", Data: startData}
				}

			case "response.output_item.added":
				if event.Item.Type == "message" && !itemStarted {
					itemStarted = true
					blockData, _ := json.Marshal(map[string]any{
						"type":          "content_block_start",
						"index":         0,
						"content_block": map[string]any{"type": "text", "text": ""},
					})
					ch <- SSEEvent{Event: "content_block_start", Data: blockData}
				}

			case "response.content_part.added":
				if event.Part.Type == "output_text" {
					partStarted = true
				}

			case "response.output_text.delta":
				if event.Delta != "" {
					deltaData, _ := json.Marshal(map[string]any{
						"type":  "content_block_delta",
						"index": 0,
						"delta": map[string]any{"type": "text_delta", "text": event.Delta},
					})
					ch <- SSEEvent{Event: "content_block_delta", Data: deltaData}
				}

			case "response.content_part.done":
				if partStarted {
					ch <- SSEEvent{Event: "content_block_stop", Data: []byte(`{"type":"content_block_stop","index":0}`)}
					partStarted = false
				}

			case "response.output_item.done":
				if itemStarted {
					itemStarted = false
				}

			case "response.completed":
				if !completed {
					// Emit message_delta with stop_reason before message_stop.
					msgDelta, _ := json.Marshal(map[string]any{
						"type":  "message_delta",
						"delta": map[string]any{"stop_reason": "end_turn"},
					})
					ch <- SSEEvent{Event: "message_delta", Data: msgDelta}
					ch <- SSEEvent{Event: "message_stop", Data: []byte(`{"type":"message_stop"}`)}
					completed = true
				}
			}
		}
		if started && !completed {
			msgDelta, _ := json.Marshal(map[string]any{
				"type":  "message_delta",
				"delta": map[string]any{"stop_reason": "end_turn"},
			})
			ch <- SSEEvent{Event: "message_delta", Data: msgDelta}
			ch <- SSEEvent{Event: "message_stop", Data: []byte(`{"type":"message_stop"}`)}
		}
	}()
	return ch
}

func (t *AnthToRes) UpdateSession(s *session.Session, _ *Request, resp *Response) {
	if resp.ReasoningContent != "" {
		s.ReasoningRecords = append(s.ReasoningRecords, session.Reasoning{
			Content: resp.ReasoningContent,
		})
	}
}
