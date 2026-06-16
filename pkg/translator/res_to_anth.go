package translator

import (
	"context"
	"encoding/json"
	"io"

	"github.com/tianyuansun/ai-gateway/pkg/schema/anthropic"
	"github.com/tianyuansun/ai-gateway/pkg/schema/responses"
	"github.com/tianyuansun/ai-gateway/pkg/session"
	"github.com/tianyuansun/ai-gateway/pkg/shared"
)

// ResToAnth translates OpenAI Responses API requests to Anthropic Messages API.
type ResToAnth struct{}

func (t *ResToAnth) TranslateRequest(_ context.Context, req *Request, s *session.Session) (*UpstreamRequest, error) {
	var body responses.ResponseRequest
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

func (t *ResToAnth) buildMessages(s *session.Session, body *responses.ResponseRequest) *anthropic.MessageRequest {
	req := &anthropic.MessageRequest{
		MaxTokens: 32768,
	}

	if body.Reasoning != nil && body.Reasoning.Effort != nil && *body.Reasoning.Effort == "high" {
		req.Thinking = &anthropic.ThinkingConfig{Type: "enabled", BudgetTokens: 16000}
	}

	_ = s // session is used only for provider affinity, not message history
	for _, item := range body.Input.Items {
		switch item.Type {
		case "message":
			text := extractInputText(item.Content)
			if item.Role == "system" || item.Role == "developer" {
				req.System = &anthropic.SystemContent{String: &text}
			} else {
				req.Messages = append(req.Messages, anthropic.MessageParam{
					Role:    item.Role,
					Content: []anthropic.ContentBlockParam{{Type: "text", Text: text}},
				})
			}
		case "function_call":
			req.Messages = append(req.Messages, anthropic.MessageParam{
				Role: "assistant",
				Content: []anthropic.ContentBlockParam{{
					Type:  "tool_use",
					ID:    item.CallID,
					Name:  item.Name,
					Input: json.RawMessage(item.Arguments),
				}},
			})
		case "function_call_output":
			req.Messages = append(req.Messages, anthropic.MessageParam{
				Role: "user",
				Content: []anthropic.ContentBlockParam{{
					Type:      "tool_result",
					ToolUseID: item.CallID,
					Content:   item.Output,
				}},
			})
	}
	}

	if len(body.Tools) > 0 {
		req.Tools = make([]anthropic.ToolDefinition, 0, len(body.Tools))
		for _, tool := range body.Tools {
			if tool.Name == "" || len(tool.Parameters) == 0 {
				continue
			}
			req.Tools = append(req.Tools, anthropic.ToolDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				InputSchema: tool.Parameters,
			})
		}
	}

	return req
}


// evt adds a monotonically increasing sequence number to an event map.
func evt(seq *int64, m map[string]any) ([]byte, error) {
	*seq++
	m["sequence_number"] = *seq
	b, err := json.Marshal(m)
	return b, err
}

func (t *ResToAnth) TranslateStream(_ context.Context, upstream io.Reader, _ *Request, _ *session.Session) <-chan SSEEvent {
	ch := make(chan SSEEvent)
	go func() {
		defer close(ch)
		seq := int64(0)
		started := false
		completed := false
		itemStarted := false
		thinkingStarted := false
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
				Index        int `json:"index"`
				ContentBlock struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content_block"`
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
					Thinking string `json:"thinking"`
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
				startData, _ := evt(&seq, map[string]any{
					"type": "response.created",
					"response": map[string]any{
						"id": responseID, "object": "response",
					},
				})
				ch <- SSEEvent{Event: "response.created", Data: startData}

				inProgressData, _ := evt(&seq, map[string]any{
					"type": "response.in_progress",
					"response": map[string]any{
						"id": responseID, "object": "response",
					},
				})
				ch <- SSEEvent{Event: "response.in_progress", Data: inProgressData}
				started = true

			case "content_block_start":
				if event.ContentBlock.Type == "text" && !itemStarted {
					itemStarted = true
					itemData, _ := evt(&seq, map[string]any{
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

					partData, _ := evt(&seq, map[string]any{
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
				if event.Delta.Type == "thinking_delta" && event.Delta.Thinking != "" {
					if !thinkingStarted {
						thinkingStarted = true
						reasoningItem, _ := evt(&seq, map[string]any{
							"type":         "response.output_item.added",
							"output_index": 1,
							"item": map[string]any{
								"id": responseID + "_reasoning", "type": "reasoning", "status": "in_progress",
							},
						})
						ch <- SSEEvent{Event: "response.output_item.added", Data: reasoningItem}
						summaryPart, _ := evt(&seq, map[string]any{
							"type":          "response.reasoning_summary_part.added",
							"item_id":       responseID + "_reasoning",
							"output_index":  1,
							"summary_index": 0,
						})
						ch <- SSEEvent{Event: "response.reasoning_summary_part.added", Data: summaryPart}
					}
					reasoningDelta, _ := evt(&seq, map[string]any{
						"type":          "response.reasoning_summary_text.delta",
						"item_id":       responseID + "_reasoning",
						"output_index":  1,
						"summary_index": 0,
						"delta":         event.Delta.Thinking,
					})
					ch <- SSEEvent{Event: "response.reasoning_summary_text.delta", Data: reasoningDelta}
				}
				if event.Delta.Type == "text_delta" && event.Delta.Text != "" {
					deltaData, _ := evt(&seq, map[string]any{
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
					partDone, _ := evt(&seq, map[string]any{
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
				if thinkingStarted {
					reasoningDone, _ := evt(&seq, map[string]any{
						"type":          "response.reasoning_summary_part.done",
						"item_id":       responseID + "_reasoning",
						"output_index":  1,
						"summary_index": 0,
					})
					ch <- SSEEvent{Event: "response.reasoning_summary_part.done", Data: reasoningDone}
					reasoningItemDone, _ := evt(&seq, map[string]any{
						"type":         "response.output_item.done",
						"output_index": 1,
						"item": map[string]any{
							"id": responseID + "_reasoning", "type": "reasoning", "status": "completed",
						},
					})
					ch <- SSEEvent{Event: "response.output_item.done", Data: reasoningItemDone}
				}
				if itemStarted {
					itemDone, _ := evt(&seq, map[string]any{
						"type":         "response.output_item.done",
						"output_index": 0,
						"item": map[string]any{
							"id":     responseID + "_item",
							"type":   "message",
							"role":   "assistant",
							"status": "completed",
						},
					})
					ch <- SSEEvent{Event: "response.output_item.done", Data: itemDone}
				}
				completed = true
				completeData, _ := evt(&seq, map[string]any{
					"type": "response.completed",
					"response": map[string]any{
						"id": responseID, "object": "response",
					},
				})
				ch <- SSEEvent{Event: "response.completed", Data: completeData}
			}
		}
		if started && !completed {
			lastData, _ := evt(&seq, map[string]any{
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

func (t *ResToAnth) convertToResponse(anthResp *anthropic.MessageResponse) *responses.Response {
	output := []responses.ResponseOutputItem{}

	for _, c := range anthResp.Content {
		switch c.Type {
		case "text":
			output = append(output, responses.ResponseOutputItem{
				Type:    "message",
				Role:    "assistant",
				Content: []responses.ResponseContentPart{{Type: "output_text", Text: c.Text}},
			})
		case "tool_use":
			args, _ := json.Marshal(c.Input)
			output = append(output, responses.ResponseOutputItem{
				Type:      "function_call",
				CallID:    c.ID,
				Name:      c.Name,
				Arguments: string(args),
			})
		}
	}

	return &responses.Response{
		ID:     anthResp.ID,
		Object: "response",
		Output: output,
		Usage:  t.convertUsage(anthResp.Usage),
	}
}

func (t *ResToAnth) convertUsage(u anthropic.Usage) *responses.ResponseUsage {
	return &responses.ResponseUsage{
		InputTokens:  u.InputTokens,
		OutputTokens: u.OutputTokens,
		TotalTokens:  u.InputTokens + u.OutputTokens,
	}
}

// extractInputText extracts text content from a ResponseInputItem's Content field,
// which can be a plain string or an array of content parts.
func extractInputText(content json.RawMessage) string {
	if len(content) == 0 {
		return ""
	}
	// Try as string first.
	var s string
	if json.Unmarshal(content, &s) == nil {
		return s
	}
	// Try as array of content parts.
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(content, &parts) == nil {
		var text string
		for _, p := range parts {
			if p.Text != "" {
				text += p.Text
			}
		}
		return text
	}
	return ""
}
