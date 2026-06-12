package translator

import (
	"context"
	"encoding/json"
	"io"

	"github.com/tianyuansun/ai-gateway/pkg/session"
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
		data, _ := io.ReadAll(upstream)
		ch <- SSEEvent{Data: data}
	}()
	return ch
}

func (t *AnthToChat) TranslateResponse(_ context.Context, upstreamBody []byte, _ *Request, _ *session.Session) (*Response, error) {
	var chatResp ChatResponse
	if err := json.Unmarshal(upstreamBody, &chatResp); err != nil {
		return nil, err
	}

	anthResp := t.convertToAnthropic(&chatResp)
	respBody, _ := json.Marshal(anthResp)
	return &Response{StatusCode: 200, Body: respBody}, nil
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

func (t *AnthToChat) UpdateSession(_ *session.Session, _ *Request, _ *Response) {}
