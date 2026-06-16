package translator

import (
	"context"
	"encoding/json"
	"io"

	"github.com/tianyuansun/ai-gateway/pkg/schema/anthropic"
	"github.com/tianyuansun/ai-gateway/pkg/schema/chat"
	"github.com/tianyuansun/ai-gateway/pkg/session"
	"github.com/tianyuansun/ai-gateway/pkg/shared"
)

// AnthToChat translates Anthropic Messages API requests to Chat Completions API.
// Used as fallback when upstream provider has only a Chat endpoint.
type AnthToChat struct{}

func (t *AnthToChat) TranslateRequest(_ context.Context, req *Request, s *session.Session) (*UpstreamRequest, error) {
	var anthReq anthropic.MessageRequest
	if err := json.Unmarshal(req.Body, &anthReq); err != nil {
		return nil, err
	}

	messages := []chat.ChatCompletionMessage{}
	if anthReq.System != nil && anthReq.System.String != nil {
		sysStr := *anthReq.System.String
		messages = append(messages, chat.ChatCompletionMessage{
			Role:    "system",
			Content: &chat.ChatCompletionMessageContent{String: &sysStr},
		})
	}
	for _, msg := range anthReq.Messages {
		cm := chat.ChatCompletionMessage{Role: msg.Role}
		var textContent string
		for _, c := range msg.Content {
			switch c.Type {
			case "text":
				textContent = c.Text
			case "tool_use":
				args, _ := json.Marshal(c.Input)
				cm.ToolCalls = append(cm.ToolCalls, chat.ChatCompletionMessageToolCall{
					ID:   c.ID,
					Type: "function",
					Function: chat.ChatCompletionToolCallFunction{
						Name:      c.Name,
						Arguments: string(args),
					},
				})
			case "tool_result":
				cm = chat.ChatCompletionMessage{
					Role:       "tool",
					Content:    &chat.ChatCompletionMessageContent{String: &c.Content},
					ToolCallID: c.ToolUseID,
				}
			}
		}
		if textContent != "" {
			cm.Content = &chat.ChatCompletionMessageContent{String: &textContent}
		}
		messages = append(messages, cm)
	}

	chatReq := chat.ChatCompletionRequest{
		Model:    req.Model,
		Messages: messages,
		Stream:   true,
	}
	if len(anthReq.Tools) > 0 {
		chatReq.Tools = make([]chat.ChatCompletionTool, len(anthReq.Tools))
		for i, t := range anthReq.Tools {
			chatReq.Tools[i] = chat.ChatCompletionTool{
				Type: "function",
				Function: chat.FunctionDefinition{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.InputSchema,
				},
			}
		}
	}
	if anthReq.Thinking != nil {
		high := "high"
		chatReq.ReasoningEffort = &high
	}

	chatBody, _ := json.Marshal(chatReq)
	return &UpstreamRequest{
		Method:  "POST",
		URL:     "/chat/completions",
		Body:    chatBody,
		Headers: map[string]string{"Content-Type": "application/json"},
	}, nil
}

func (t *AnthToChat) TranslateStream(_ context.Context, upstream io.Reader, _ *Request, _ *session.Session) <-chan SSEEvent {
	ch := make(chan SSEEvent)
	go func() {
		defer close(ch)
		started := false
		var msgID string
		for sseEv := range shared.ParseSSE(upstream) {
			if sseEv.Data == "[DONE]" {
				ch <- SSEEvent{Event: "message_stop", Data: []byte(`{"type":"message_stop"}`)}
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
				msgID = chunk.ID
			}

			if !started && msgID != "" {
				started = true
				startData, _ := json.Marshal(map[string]any{
					"type":    "message_start",
					"message": map[string]any{"id": msgID, "type": "message", "role": "assistant"},
				})
				ch <- SSEEvent{Event: "message_start", Data: startData}

				blockData, _ := json.Marshal(map[string]any{
					"type":          "content_block_start",
					"index":         0,
					"content_block": map[string]any{"type": "text", "text": ""},
				})
				ch <- SSEEvent{Event: "content_block_start", Data: blockData}
			}

			for _, choice := range chunk.Choices {
				if choice.Delta.Content != "" {
					deltaData, _ := json.Marshal(map[string]any{
						"type":  "content_block_delta",
						"index": 0,
						"delta": map[string]any{"type": "text_delta", "text": choice.Delta.Content},
					})
					ch <- SSEEvent{Event: "content_block_delta", Data: deltaData}
				}
				if choice.FinishReason != nil {
					ch <- SSEEvent{Event: "content_block_stop", Data: []byte(`{"type":"content_block_stop","index":0}`)}
					ch <- SSEEvent{Event: "message_delta", Data: []byte(`{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`)}
					ch <- SSEEvent{Event: "message_stop", Data: []byte(`{"type":"message_stop"}`)}
				}
			}
		}
	}()
	return ch
}

func (t *AnthToChat) convertToAnthropic(chatResp *chat.ChatCompletion) *anthropic.MessageResponse {
	var content []anthropic.ResponseContentBlock
	msg := chatResp.Choices[0].Message

	if msg.Content != nil && msg.Content.String != nil {
		content = append(content, anthropic.ResponseContentBlock{Type: "text", Text: *msg.Content.String})
	}
	for _, tc := range msg.ToolCalls {
		content = append(content, anthropic.ResponseContentBlock{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: json.RawMessage(tc.Function.Arguments),
		})
	}

	return &anthropic.MessageResponse{
		ID:      chatResp.ID,
		Type:    "message",
		Role:    "assistant",
		Content: content,
		Usage: anthropic.Usage{
			InputTokens:  chatResp.Usage.PromptTokens,
			OutputTokens: chatResp.Usage.CompletionTokens,
		},
	}
}
