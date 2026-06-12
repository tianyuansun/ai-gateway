package translator

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/tianyuansun/ai-gateway/pkg/session"
	"github.com/tianyuansun/ai-gateway/pkg/shared"
)

// ResToAnth translates OpenAI Responses API requests to Anthropic Messages API.
type ResToAnth struct{}

func (t *ResToAnth) TranslateRequest(_ context.Context, req *Request, s *session.Session) (*UpstreamRequest, error) {
	var body ResponsesRequest
	if err := json.Unmarshal(req.Body, &body); err != nil {
		return nil, err
	}

	anthReq := t.buildMessages(s, &body)
	anthReq.Model = req.Model
	anthReq.Stream = true

	anthBody, err := json.Marshal(anthReq)
	if err != nil {
		return nil, err
	}

	return &UpstreamRequest{
		Method: "POST",
		URL:    "/messages",
		Body:   anthBody,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}

func (t *ResToAnth) buildMessages(s *session.Session, body *ResponsesRequest) *AnthropicRequest {
	req := &AnthropicRequest{
		MaxTokens: 32768,
	}

	if body.Reasoning != nil && body.Reasoning.Effort == "high" {
		req.Thinking = &ThinkingConfig{Type: "enabled", BudgetTokens: 16000}
	}

	// Rebuild full history from session when available (mirrors ResToChat pattern).
	if s != nil && len(s.Messages) > 0 {
		req.Messages = sessionToAnthropicMessagesWithReasoning(s.Messages, s.ReasoningRecords)
	} else {
		for _, item := range body.Input {
			switch item.Type {
			case "message":
				if item.Role == "system" || item.Role == "developer" {
					req.System = item.extractText()
				} else {
					req.Messages = append(req.Messages, AnthropicMessage{
						Role:    item.Role,
						Content: []AnthropicContent{{Type: "text", Text: item.extractText()}},
					})
				}
			case "function_call":
				req.Messages = append(req.Messages, AnthropicMessage{
					Role: "assistant",
					Content: []AnthropicContent{{
						Type:  "tool_use",
						ID:    item.CallID,
						Name:  item.Name,
						Input: parseJSON(item.Arguments),
					}},
				})
			case "function_call_output":
				req.Messages = append(req.Messages, AnthropicMessage{
					Role: "user",
					Content: []AnthropicContent{{
						Type:        "tool_result",
						ToolUseID:   item.CallID,
						Content:     item.Output,
					}},
				})
			}
		}
	}

	if len(body.Tools) > 0 {
		req.Tools = make([]AnthropicTool, 0, len(body.Tools))
		for _, tool := range body.Tools {
			// Skip tools with empty names or null input schemas (invalid for Anthropic API).
			if tool.Name == "" || tool.Parameters == nil {
				continue
			}
			req.Tools = append(req.Tools, AnthropicTool{
				Name:        tool.Name,
				Description: tool.Description,
				InputSchema: tool.Parameters,
			})
		}
	}

	return req
}

func sessionToAnthropicMessagesWithReasoning(msgs []session.Message, reasoningRecords []session.Reasoning) []AnthropicMessage {
	var result []AnthropicMessage
	for _, m := range msgs {
		switch m.Role {
		case "tool":
			result = append(result, AnthropicMessage{
				Role: "user",
				Content: []AnthropicContent{{
					Type:      "tool_result",
					ToolUseID: m.ToolCallID,
					Content:   m.Content,
				}},
			})
		case "assistant":
			var content []AnthropicContent
			if m.Content != "" {
				content = append(content, AnthropicContent{Type: "text", Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				content = append(content, AnthropicContent{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: parseJSON(tc.Function.Arguments),
				})
			}
			result = append(result, AnthropicMessage{Role: "assistant", Content: content})
		default:
			result = append(result, AnthropicMessage{
				Role:    m.Role,
				Content: []AnthropicContent{{Type: "text", Text: m.Content}},
			})
		}
	}
	// Inject reasoning as thinking blocks into the last assistant message.
	if len(reasoningRecords) > 0 {
		for i := len(result) - 1; i >= 0; i-- {
			if result[i].Role == "assistant" {
				thinkingBlock := AnthropicContent{
					Type:      "thinking",
					Thinking:  reasoningRecords[len(reasoningRecords)-1].Content,
					Signature: "",
				}
				result[i].Content = append([]AnthropicContent{thinkingBlock}, result[i].Content...)
				break
			}
		}
	}
	return result
}

func parseJSON(s string) any {
	var v any
	json.Unmarshal([]byte(s), &v)
	return v
}

func (t *ResToAnth) TranslateStream(_ context.Context, upstream io.Reader, _ *Request, _ *session.Session) <-chan SSEEvent {
	ch := make(chan SSEEvent)
	go func() {
		defer close(ch)
		started := false
		completed := false
		itemStarted := false
		var responseID string
		for sseEv := range shared.ParseSSE(upstream) {
			var event struct {
				Type    string `json:"type"`
				Message struct {
					ID    string `json:"id"`
					Model string `json:"model"`
					Usage struct {
						InputTokens int `json:"input_tokens"`
					} `json:"usage"`
				} `json:"message"`
				Index       int `json:"index"`
				ContentBlock struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content_block"`
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
				Usage struct {
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			}
			if err := json.Unmarshal([]byte(sseEv.Data), &event); err != nil {
				continue
			}

			switch event.Type {
			case "message_start":
				responseID = event.Message.ID
				startData, _ := json.Marshal(map[string]any{
					"type": "response.created",
					"response": map[string]any{
						"id": responseID, "object": "response",
					},
				})
				ch <- SSEEvent{Event: "response.created", Data: startData}
				started = true

			case "content_block_start":
				if event.ContentBlock.Type == "text" && !itemStarted {
					itemStarted = true
					itemData, _ := json.Marshal(map[string]any{
						"type":         "response.output_item.added",
						"output_index": 0,
						"item": map[string]any{
							"id":     responseID + "_item",
							"type":   "message",
							"role":   "assistant",
							"status": "in_progress",
						},
					})
					ch <- SSEEvent{Event: "response.output_item.added", Data: itemData}

					partData, _ := json.Marshal(map[string]any{
						"type":          "response.content_part.added",
						"item_id":       responseID + "_item",
						"output_index":  0,
						"content_index": 0,
						"part": map[string]any{
							"type": "output_text",
							"text": "",
						},
					})
					ch <- SSEEvent{Event: "response.content_part.added", Data: partData}
				}

			case "content_block_delta":
				if event.Delta.Type == "text_delta" && event.Delta.Text != "" {
					deltaData, _ := json.Marshal(map[string]any{
						"type":          "response.output_text.delta",
						"item_id":       responseID + "_item",
						"output_index":  0,
						"content_index": 0,
						"delta":         event.Delta.Text,
					})
					ch <- SSEEvent{Event: "response.output_text.delta", Data: deltaData}
				}

			case "content_block_stop":
				if itemStarted {
					partDone, _ := json.Marshal(map[string]any{
						"type":          "response.content_part.done",
						"item_id":       responseID + "_item",
						"output_index":  0,
						"content_index": 0,
						"part": map[string]any{
							"type": "output_text",
							"text": "",
						},
					})
					ch <- SSEEvent{Event: "response.content_part.done", Data: partDone}
				}

			case "message_stop":
				if itemStarted {
					itemDone, _ := json.Marshal(map[string]any{
						"type":         "response.output_item.done",
						"output_index": 0,
						"item": map[string]any{
							"id": responseID + "_item",
							"type": "message",
							"role": "assistant", "status": "completed",
						},
					})
					ch <- SSEEvent{Event: "response.output_item.done", Data: itemDone}
				}
				completed = true
				completeData, _ := json.Marshal(map[string]any{
					"type": "response.completed",
					"response": map[string]any{
						"id": responseID, "object": "response",
					},
				})
				ch <- SSEEvent{Event: "response.completed", Data: completeData}
			}
		}
		if started && !completed {
			lastData, _ := json.Marshal(map[string]any{
				"type": "response.completed",
				"response": map[string]any{
					"id": responseID, "object": "response",
				},
			})
			ch <- SSEEvent{Event: "response.completed", Data: lastData}
		}
	}()
	return ch
}

func (t *ResToAnth) TranslateResponse(_ context.Context, upstream *http.Response, _ *Request, _ *session.Session) (*Response, error) {
	body, err := io.ReadAll(upstream.Body)
	if err != nil {
		return nil, err
	}
	upstream.Body.Close()

	var anthResp AnthropicResponse
	if err := json.Unmarshal(body, &anthResp); err != nil {
		return nil, err
	}

	reasoningContent := ""
	for _, c := range anthResp.Content {
		if c.Type == "thinking" && c.Thinking != "" {
			reasoningContent += c.Thinking
		}
	}

	resp := t.convertToResponse(&anthResp)

	respBody, _ := json.Marshal(resp)
	return &Response{StatusCode: 200, Body: respBody, ReasoningContent: reasoningContent}, nil
}

func (t *ResToAnth) convertToResponse(anthResp *AnthropicResponse) *ResponsesResponse {
	output := []OutputItem{}

	for _, c := range anthResp.Content {
		switch c.Type {
		case "text":
			output = append(output, OutputItem{
				Type: "message",
				Role: "assistant",
				Content: []ContentPart{{Type: "output_text", Text: c.Text}},
			})
		case "tool_use":
			args, _ := json.Marshal(c.Input)
			output = append(output, OutputItem{
				Type:      "function_call",
				CallID:    c.ID,
				Name:      c.Name,
				Arguments: string(args),
			})
		}
	}

	return &ResponsesResponse{
		ID:     anthResp.ID,
		Object: "response",
		Output: output,
		Usage:  t.convertUsage(anthResp.Usage),
	}
}

func (t *ResToAnth) convertUsage(u *AnthropicUsage) *Usage {
	if u == nil {
		return nil
	}
	return &Usage{
		PromptTokens:     u.InputTokens,
		CompletionTokens: u.OutputTokens,
		TotalTokens:      u.InputTokens + u.OutputTokens,
	}
}

func (t *ResToAnth) UpdateSession(s *session.Session, _ *Request, resp *Response) {
	if resp.ReasoningContent != "" {
		s.ReasoningRecords = append(s.ReasoningRecords, session.Reasoning{
			Content: resp.ReasoningContent,
		})
	}
}

// --- Anthropic types ---

type AnthropicRequest struct {
	Model     string             `json:"model"`
	System    string             `json:"system,omitempty"`
	Messages  []AnthropicMessage `json:"messages"`
	Tools     []AnthropicTool    `json:"tools,omitempty"`
	MaxTokens int                `json:"max_tokens"`
	Stream    bool               `json:"stream,omitempty"`
	Thinking  *ThinkingConfig    `json:"thinking,omitempty"`
}

type AnthropicMessage struct {
	Role    string             `json:"role"`
	Content []AnthropicContent `json:"content"`
}

type AnthropicContent struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	Thinking  string `json:"thinking,omitempty"`
	Signature string `json:"signature,omitempty"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Input     any    `json:"input,omitempty"`
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
}

type AnthropicTool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"input_schema"`
}

type ThinkingConfig struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens"`
}

type AnthropicResponse struct {
	ID      string             `json:"id"`
	Type    string             `json:"type"`
	Role    string             `json:"role"`
	Content []AnthropicContent `json:"content"`
	Usage   *AnthropicUsage    `json:"usage,omitempty"`
}

type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}
