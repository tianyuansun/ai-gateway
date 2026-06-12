package translator

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/tianyuansun/ai-gateway/pkg/session"
	"github.com/tianyuansun/ai-gateway/pkg/shared"
)

// AnthToChat translates Anthropic Messages API requests to Chat Completions API.
// Used as fallback when upstream provider has only a Chat endpoint.
type AnthToChat struct{}

func (t *AnthToChat) TranslateRequest(_ context.Context, req *Request, s *session.Session) (*UpstreamRequest, error) {
	var anthReq AnthropicRequest
	if err := json.Unmarshal(req.Body, &anthReq); err != nil {
		return nil, err
	}

	messages := []ChatMessage{}
	if anthReq.System != "" {
		messages = append(messages, ChatMessage{Role: "system", Content: anthReq.System})
	}
	for _, msg := range anthReq.Messages {
		cm := ChatMessage{Role: msg.Role}
		for _, c := range msg.Content {
			switch c.Type {
			case "text":
				cm.Content = c.Text
			case "tool_use":
				args, _ := json.Marshal(c.Input)
				cm.ToolCalls = append(cm.ToolCalls, ChatToolCall{
					ID:       c.ID,
					Type:     "function",
					Function: session.FunctionCall{Name: c.Name, Arguments: string(args)},
				})
			case "tool_result":
				cm = ChatMessage{
					Role:       "tool",
					Content:    c.Content,
					ToolCallID: c.ToolUseID,
				}
			}
		}
		messages = append(messages, cm)
	}

	chatReq := ChatRequest{
		Model:    req.Model,
		Messages: messages,
		Stream:   true,
	}
	if len(anthReq.Tools) > 0 {
		chatReq.Tools = make([]Tool, len(anthReq.Tools))
		for i, t := range anthReq.Tools {
			chatReq.Tools[i] = Tool{
				Type:       "function",
				Name:       t.Name,
				Description: t.Description,
				Parameters: t.InputSchema,
			}
		}
	}
	if anthReq.Thinking != nil {
		chatReq.ReasoningEffort = "high"
	}

	chatBody, _ := json.Marshal(chatReq)
	return &UpstreamRequest{
		Method: "POST",
		URL:    "/chat/completions",
		Body:   chatBody,
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

func (t *AnthToChat) TranslateResponse(_ context.Context, upstream *http.Response, _ *Request, _ *session.Session) (*Response, error) {
	body, err := io.ReadAll(upstream.Body)
	if err != nil {
		return nil, err
	}
	upstream.Body.Close()

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, err
	}

	reasoningContent := extractReasoningContent(body)

	anthResp := t.convertToAnthropic(&chatResp)
	respBody, _ := json.Marshal(anthResp)
	return &Response{StatusCode: 200, Body: respBody, ReasoningContent: reasoningContent}, nil
}

func (t *AnthToChat) convertToAnthropic(chatResp *ChatResponse) *AnthropicResponse {
	var content []AnthropicContent
	msg := chatResp.Choices[0].Message

	if msg.Content != "" {
		content = append(content, AnthropicContent{Type: "text", Text: msg.Content})
	}
	for _, tc := range msg.ToolCalls {
		input := parseJSON(tc.Function.Arguments)
		content = append(content, AnthropicContent{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: input,
		})
	}

	return &AnthropicResponse{
		ID:      chatResp.ID,
		Type:    "message",
		Role:    "assistant",
		Content: content,
		Usage: &AnthropicUsage{
			InputTokens:  chatResp.Usage.PromptTokens,
			OutputTokens: chatResp.Usage.CompletionTokens,
		},
	}
}

func (t *AnthToChat) UpdateSession(s *session.Session, _ *Request, resp *Response) {
	if resp.ReasoningContent != "" {
		s.ReasoningRecords = append(s.ReasoningRecords, session.Reasoning{
			Content: resp.ReasoningContent,
		})
	}
}
