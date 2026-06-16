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

// ChatToAnth translates Chat Completions API requests to Anthropic Messages API.
// Used as fallback when upstream provider has only an Anthropic endpoint.
type ChatToAnth struct{}

func (t *ChatToAnth) TranslateRequest(_ context.Context, req *Request, s *session.Session) (*UpstreamRequest, error) {
	var chatReq chat.ChatCompletionRequest
	if err := json.Unmarshal(req.Body, &chatReq); err != nil {
		return nil, err
	}

	anthReq := &anthropic.MessageRequest{
		Model:     req.Model,
		MaxTokens: 32768,
		Stream:    true,
	}

	messages := []anthropic.MessageParam{}
	for _, msg := range chatReq.Messages {
		msgText := contentString(msg.Content)
		switch msg.Role {
		case "system", "developer":
			anthReq.System = &anthropic.SystemContent{String: &msgText}
		case "assistant":
			var content []anthropic.ContentBlockParam
			if len(msg.ToolCalls) > 0 {
				for _, tc := range msg.ToolCalls {
					content = append(content, anthropic.ContentBlockParam{
						Type:  "tool_use",
						ID:    tc.ID,
						Name:  tc.Function.Name,
						Input: json.RawMessage(tc.Function.Arguments),
					})
				}
			} else if msgText != "" {
				content = append(content, anthropic.ContentBlockParam{Type: "text", Text: msgText})
			}
			messages = append(messages, anthropic.MessageParam{Role: "assistant", Content: content})
		case "tool":
			messages = append(messages, anthropic.MessageParam{
				Role: "user",
				Content: []anthropic.ContentBlockParam{{
					Type:      "tool_result",
					ToolUseID: msg.ToolCallID,
					Content:   msgText,
				}},
			})
		default:
			messages = append(messages, anthropic.MessageParam{
				Role:    msg.Role,
				Content: []anthropic.ContentBlockParam{{Type: "text", Text: msgText}},
			})
		}
	}
	anthReq.Messages = messages

	if len(chatReq.Tools) > 0 {
		anthReq.Tools = make([]anthropic.ToolDefinition, len(chatReq.Tools))
		for i, tool := range chatReq.Tools {
			anthReq.Tools[i] = anthropic.ToolDefinition{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				InputSchema: tool.Function.Parameters,
			}
		}
	}
	if chatReq.ReasoningEffort != nil && *chatReq.ReasoningEffort == "high" {
		anthReq.Thinking = &anthropic.ThinkingConfig{Type: "enabled", BudgetTokens: 16000}
	}

	anthBody, _ := json.Marshal(anthReq)
	return &UpstreamRequest{
		Method:  "POST",
		URL:     "/messages",
		Body:    anthBody,
		Headers: map[string]string{"Content-Type": "application/json"},
	}, nil
}

func (t *ChatToAnth) TranslateStream(_ context.Context, upstream io.Reader, _ *Request, _ *session.Session) <-chan SSEEvent {
	ch := make(chan SSEEvent)
	go func() {
		defer close(ch)
		var msgID string
		for sseEv := range shared.ParseSSE(upstream) {
			var event struct {
				Type    string `json:"type"`
				Message struct {
					ID string `json:"id"`
				} `json:"message"`
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
			}
			if err := json.Unmarshal([]byte(sseEv.Data), &event); err != nil {
				continue
			}

			switch event.Type {
			case "message_start":
				msgID = event.Message.ID

			case "content_block_delta":
				if event.Delta.Type == "text_delta" && event.Delta.Text != "" {
					chunkData, _ := json.Marshal(map[string]any{
						"id":      msgID,
						"object":  "chat.completion.chunk",
						"choices": []map[string]any{{"index": 0, "delta": map[string]any{"content": event.Delta.Text}}},
					})
					ch <- SSEEvent{Data: chunkData}
				}

			case "message_stop":
				doneData, _ := json.Marshal(map[string]any{
					"id":      msgID,
					"object":  "chat.completion.chunk",
					"choices": []map[string]any{{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"}},
				})
				ch <- SSEEvent{Data: doneData}
			}
		}
	}()
	return ch
}

// contentString extracts a plain string from ChatCompletionMessageContent.
func contentString(c *chat.ChatCompletionMessageContent) string {
	if c == nil || c.String == nil {
		return ""
	}
	return *c.String
}
