package translator

import (
	"context"
	"encoding/json"
	"io"

	"github.com/tianyuansun/ai-gateway/pkg/schema/chat"
	"github.com/tianyuansun/ai-gateway/pkg/schema/responses"
	"github.com/tianyuansun/ai-gateway/pkg/session"
	"github.com/tianyuansun/ai-gateway/pkg/shared"
)

// ResToChat translates OpenAI Responses API requests to Chat Completions API.
type ResToChat struct{}

func (t *ResToChat) TranslateRequest(_ context.Context, req *Request, s *session.Session) (*UpstreamRequest, error) {
	var body responses.ResponseRequest
	if err := json.Unmarshal(req.Body, &body); err != nil {
		return nil, err
	}

	messages := t.rebuildMessages(s, &body)

	// Forward instructions as the first system message.
	// This is used by Codex CLI local compact to inject a summarization prompt.
	if body.Instructions != nil && *body.Instructions != "" {
		messages = append([]chat.ChatCompletionMessage{{
			Role:    "system",
			Content: &chat.ChatCompletionMessageContent{String: body.Instructions},
		}}, messages...)
	}

	chatReq := chat.ChatCompletionRequest{
		Model:    req.Model,
		Messages: messages,
		Stream:   true,
	}
	if len(body.Tools) > 0 {
		chatReq.Tools = make([]chat.ChatCompletionTool, len(body.Tools))
		for i, tool := range body.Tools {
			chatReq.Tools[i] = chat.ChatCompletionTool{
				Type: "function",
				Function: chat.FunctionDefinition{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  tool.Parameters,
				},
			}
		}
	}
	if body.Reasoning != nil && body.Reasoning.Effort != nil {
		chatReq.ReasoningEffort = body.Reasoning.Effort
	}

	chatBody, err := json.Marshal(chatReq)
	if err != nil {
		return nil, err
	}

	return &UpstreamRequest{
		Method: "POST",
		URL:    "/chat/completions",
		Body:   chatBody,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}

func (t *ResToChat) rebuildMessages(s *session.Session, body *responses.ResponseRequest) []chat.ChatCompletionMessage {
	if s != nil && len(s.Messages) > 0 {
		return t.sessionMessages(s)
	}

	msgs := make([]chat.ChatCompletionMessage, 0, len(body.Input.Items))
	for _, item := range body.Input.Items {
		switch item.Type {
		case "message":
			text := extractInputText(item.Content)
			msgs = append(msgs, chat.ChatCompletionMessage{
				Role:    item.Role,
				Content: &chat.ChatCompletionMessageContent{String: &text},
			})
		case "function_call":
			msgs = append(msgs, chat.ChatCompletionMessage{
				Role: "assistant",
				ToolCalls: []chat.ChatCompletionMessageToolCall{{
					ID:   item.CallID,
					Type: "function",
					Function: chat.ChatCompletionToolCallFunction{
						Name:      item.Name,
						Arguments: item.Arguments,
					},
				}},
			})
		case "function_call_output":
			msgs = append(msgs, chat.ChatCompletionMessage{
				Role:       "tool",
				ToolCallID: item.CallID,
				Content:    &chat.ChatCompletionMessageContent{String: &item.Output},
			})
		}
	}
	return msgs
}

func (t *ResToChat) sessionMessages(s *session.Session) []chat.ChatCompletionMessage {
	msgs := make([]chat.ChatCompletionMessage, len(s.Messages))
	for i, m := range s.Messages {
		msgs[i] = chat.ChatCompletionMessage{
			Role:       m.Role,
			Content:    &chat.ChatCompletionMessageContent{String: &m.Content},
			ToolCallID: m.ToolCallID,
			Name:       m.Name,
		}
		if len(m.ToolCalls) > 0 {
			msgs[i].ToolCalls = make([]chat.ChatCompletionMessageToolCall, len(m.ToolCalls))
			for j, tc := range m.ToolCalls {
				msgs[i].ToolCalls[j] = chat.ChatCompletionMessageToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: chat.ChatCompletionToolCallFunction{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
			}
		}
	}
	// Inject reasoning records into the last assistant message.
	if len(s.ReasoningRecords) > 0 {
		for i := len(msgs) - 1; i >= 0; i-- {
			if msgs[i].Role == "assistant" {
				msgs[i].ReasoningContent = s.ReasoningRecords[len(s.ReasoningRecords)-1].Content
				break
			}
		}
	}
	return msgs
}

func (t *ResToChat) TranslateStream(_ context.Context, upstream io.Reader, _ *Request, _ *session.Session) <-chan SSEEvent {
	ch := make(chan SSEEvent)
	go func() {
		defer close(ch)
		seq := int64(0)
		started := false
		completed := false
		itemStarted := false
		var responseID string
		for sseEv := range shared.ParseSSE(upstream) {
			if sseEv.Data == "[DONE]" {
				if itemStarted {
					itemDone, _ := evt(&seq, map[string]any{
						"type":         "response.output_item.done",
						"output_index": 0,
						"item": map[string]any{
							"id": responseID + "_item", "type": "message", "role": "assistant", "status": "in_progress", "content": []any{},
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
				continue
			}

			var chunk struct {
				ID      string `json:"id"`
				Choices []struct {
					Delta struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					} `json:"delta"`
					FinishReason *string `json:"finish_reason"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(sseEv.Data), &chunk); err != nil {
				continue
			}
			if chunk.ID != "" {
				responseID = chunk.ID
			}

			if !started {
				started = true
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
			}

			for _, choice := range chunk.Choices {
				if choice.Delta.Content != "" && !itemStarted {
					itemStarted = true
					itemData, _ := evt(&seq, map[string]any{
						"type":         "response.output_item.added",
						"output_index": 0,
						"item": map[string]any{
							"id": responseID + "_item", "type": "message", "role": "assistant", "status": "in_progress", "content": []any{},
						},
					})
					ch <- SSEEvent{Event: "response.output_item.added", Data: itemData}
					partData, _ := evt(&seq, map[string]any{
						"type":          "response.content_part.added",
						"item_id":       responseID + "_item",
						"output_index":  0,
						"content_index": 0,
						"part":          map[string]any{"type": "output_text", "text": ""},
					})
					ch <- SSEEvent{Event: "response.content_part.added", Data: partData}
				}
				if choice.Delta.Content != "" {
					deltaData, _ := evt(&seq, map[string]any{
						"type":          "response.output_text.delta",
						"item_id":       responseID + "_item",
						"output_index":  0,
						"content_index": 0,
						"delta":         choice.Delta.Content,
					})
					ch <- SSEEvent{Event: "response.output_text.delta", Data: deltaData}
				}
				if choice.FinishReason != nil {
					if itemStarted {
						itemDone, _ := evt(&seq, map[string]any{
							"type":         "response.output_item.done",
							"output_index": 0,
							"item": map[string]any{
								"id": responseID + "_item", "type": "message", "role": "assistant", "status": "in_progress",
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

func (t *ResToChat) convertToResponse(chatResp *chat.ChatCompletion) *responses.Response {
	msg := chatResp.Choices[0].Message

	output := []responses.ResponseOutputItem{}
	if msg.Content != nil && msg.Content.String != nil {
		output = append(output, responses.ResponseOutputItem{
			Type: "message",
			Role: "assistant",
			Content: []responses.ResponseContentPart{{
				Type: "output_text",
				Text: *msg.Content.String,
			}},
		})
	}
	for _, tc := range msg.ToolCalls {
		output = append(output, responses.ResponseOutputItem{
			Type:      "function_call",
			CallID:    tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}

	return &responses.Response{
		ID:     chatResp.ID,
		Object: "response",
		Output: output,
		Usage:  t.convertUsage(chatResp.Usage),
	}
}

func (t *ResToChat) convertUsage(u *chat.CompletionUsage) *responses.ResponseUsage {
	if u == nil {
		return nil
	}
	return &responses.ResponseUsage{
		InputTokens:  u.PromptTokens,
		OutputTokens: u.CompletionTokens,
		TotalTokens:  u.TotalTokens,
	}
}

func (t *ResToChat) appendToSession(s *session.Session, chatResp *chat.ChatCompletion) {
	msg := chatResp.Choices[0].Message
	s.Messages = append(s.Messages, session.Message{
		Role: "assistant",
	})
	last := &s.Messages[len(s.Messages)-1]
	if msg.Content != nil && msg.Content.String != nil {
		last.Content = *msg.Content.String
	}
	if len(msg.ToolCalls) > 0 {
		last.Content = ""
		last.ToolCalls = make([]session.ToolCall, len(msg.ToolCalls))
		for i, tc := range msg.ToolCalls {
			last.ToolCalls[i] = session.ToolCall{
				ID:       tc.ID,
				Type:     tc.Type,
				Function: session.FunctionCall{Name: tc.Function.Name, Arguments: tc.Function.Arguments},
			}
		}
	}
}

// extractReasoningContent extracts reasoning_content from a Chat Completions response.
func extractReasoningContent(body []byte) string {
	var raw struct {
		Choices []struct {
			Message struct {
				ReasoningContent string `json:"reasoning_content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return ""
	}
	if len(raw.Choices) > 0 {
		return raw.Choices[0].Message.ReasoningContent
	}
	return ""
}
