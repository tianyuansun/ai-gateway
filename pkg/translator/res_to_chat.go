package translator

import (
	"context"
	"encoding/json"
	"io"
	"strings"

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
	_ = s // session is used only for provider affinity, not message history
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
		case "reasoning":
			summary := extractReasoningSummary(item.Summary)
			if summary != "" {
				for j := len(msgs) - 1; j >= 0; j-- {
					if msgs[j].Role == "assistant" {
						msgs[j].ReasoningContent = summary
						break
					}
				}
			}
		}
	}
	return msgs
}

func extractReasoningSummary(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return ""
	}
	var parts []string
	for _, b := range blocks {
		if b.Type == "summary_text" && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n")
}


func (t *ResToChat) TranslateStream(_ context.Context, upstream io.Reader, _ *Request, _ *session.Session) <-chan SSEEvent {
	ch := make(chan SSEEvent)
	go func() {
		defer close(ch)
		seq := int64(0)
		started := false
		completed := false
		itemStarted := false
		reasoningStarted := false
		var responseID string
		for sseEv := range shared.ParseSSE(upstream) {
			if sseEv.Data == "[DONE]" {
				if reasoningStarted {
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
						Role             string `json:"role"`
						Content          string `json:"content"`
						ReasoningContent string `json:"reasoning_content"`
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
				// Reasoning content: emit reasoning SSE events.
				if choice.Delta.ReasoningContent != "" {
					if !reasoningStarted {
						reasoningStarted = true
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
						"delta":         choice.Delta.ReasoningContent,
					})
					ch <- SSEEvent{Event: "response.reasoning_summary_text.delta", Data: reasoningDelta}
				}
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
					if reasoningStarted {
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




// extractReasoningContent extracts reasoning_content from a Chat Completions response.
