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

// ChatToRes translates Chat Completions API requests to OpenAI Responses API.
type ChatToRes struct{}

func (t *ChatToRes) TranslateRequest(_ context.Context, req *Request, s *session.Session) (*UpstreamRequest, error) {
	var chatReq chat.ChatCompletionRequest
	if err := json.Unmarshal(req.Body, &chatReq); err != nil {
		return nil, err
	}

	// Build instructions from system messages.
	var instructionsChat *string
	var inputItems []responses.ResponseInputItem

	for _, msg := range chatReq.Messages {
		switch msg.Role {
		case "system":
			content := contentString(msg.Content)
			if instructionsChat == nil {
				instructionsChat = &content
			} else {
				combined := *instructionsChat + "\n" + content
				instructionsChat = &combined
			}
		case "user":
			content := contentString(msg.Content)
			contentJSON, _ := json.Marshal(content)
			inputItems = append(inputItems, responses.ResponseInputItem{
				Type:    "message",
				Role:    "user",
				Content: json.RawMessage(contentJSON),
			})
		case "assistant":
			if len(msg.ToolCalls) > 0 {
				for _, tc := range msg.ToolCalls {
					inputItems = append(inputItems, responses.ResponseInputItem{
						Type:      "function_call",
						CallID:    tc.ID,
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					})
				}
			} else {
				content := contentString(msg.Content)
				contentJSON, _ := json.Marshal(content)
				inputItems = append(inputItems, responses.ResponseInputItem{
					Type:    "message",
					Role:    "assistant",
					Content: json.RawMessage(contentJSON),
				})
			}
		case "tool":
			content := contentString(msg.Content)
			inputItems = append(inputItems, responses.ResponseInputItem{
				Type:   "function_call_output",
				CallID: msg.ToolCallID,
				Output: content,
			})
		}
	}

	respReq := responses.ResponseRequest{
		Model:        req.Model,
		Instructions: instructionsChat,
		Stream:       chatReq.Stream,
	}

	// Map temperature, top_p, max_tokens.
	respReq.Temperature = chatReq.Temperature
	respReq.TopP = chatReq.TopP
	if chatReq.MaxTokens != nil {
		respReq.MaxOutputTokens = chatReq.MaxTokens
	}

	// Map tools.
	if len(chatReq.Tools) > 0 {
		respReq.Tools = make([]responses.ToolDefinition, len(chatReq.Tools))
		for i, tool := range chatReq.Tools {
			respReq.Tools[i] = responses.ToolDefinition{
				Type:        tool.Type,
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				Parameters:  tool.Function.Parameters,
			}
		}
	}

	// Map reasoning_effort.
	if chatReq.ReasoningEffort != nil {
		respReq.Reasoning = &responses.ReasoningConfig{
			Effort: chatReq.ReasoningEffort,
		}
	}

	// Set input.
	if len(inputItems) > 0 {
		respReq.Input = responses.ResponseInput{Items: inputItems}
	}

	respBody, err := json.Marshal(respReq)
	if err != nil {
		return nil, err
	}

	return &UpstreamRequest{
		Method: "POST",
		URL:    "/responses",
		Body:   respBody,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}

func (t *ChatToRes) convertToChatCompletion(resp *responses.Response) *chat.ChatCompletion {
	var content *chat.ChatCompletionMessageContent
	var toolCalls []chat.ChatCompletionMessageToolCall

	for _, item := range resp.Output {
		switch item.Type {
		case "message":
			text := extractOutputText(item.Content)
			if text != "" {
				content = &chat.ChatCompletionMessageContent{String: &text}
			}
		case "function_call":
			toolCalls = append(toolCalls, chat.ChatCompletionMessageToolCall{
				ID:   item.CallID,
				Type: "function",
				Function: chat.ChatCompletionToolCallFunction{
					Name:      item.Name,
					Arguments: item.Arguments,
				},
			})
		}
	}

	msg := chat.ChatCompletionMessage{
		Role:      "assistant",
		Content:   content,
		ToolCalls: toolCalls,
	}

	finishReason := "stop"
	return &chat.ChatCompletion{
		ID:      resp.ID,
		Object:  "chat.completion",
		Model:   resp.Model,
		Choices: []chat.ChatCompletionChoice{{Index: 0, Message: msg, FinishReason: &finishReason}},
		Usage:   t.convertUsage(resp.Usage),
	}
}

func (t *ChatToRes) convertUsage(u *responses.ResponseUsage) *chat.CompletionUsage {
	if u == nil {
		return nil
	}
	return &chat.CompletionUsage{
		PromptTokens:     u.InputTokens,
		CompletionTokens: u.OutputTokens,
		TotalTokens:      u.TotalTokens,
	}
}


func (t *ChatToRes) extractReasoningFromResponse(resp *responses.Response) string {
	for _, item := range resp.Output {
		if item.Type == "reasoning" {
			var allText string
			for _, summary := range item.Summary {
				var s struct {
					Text string `json:"text"`
				}
				if json.Unmarshal(summary, &s) == nil && s.Text != "" {
					allText += s.Text
				}
			}
			return allText
		}
	}
	return ""
}

// extractOutputText extracts text content from a ResponseOutputItem's Content array.
func extractOutputText(parts []responses.ResponseContentPart) string {
	var text string
	for _, p := range parts {
		if p.Text != "" {
			text += p.Text
		}
	}
	return text
}

func (t *ChatToRes) TranslateStream(_ context.Context, upstream io.Reader, _ *Request, _ *session.Session) <-chan SSEEvent {
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
