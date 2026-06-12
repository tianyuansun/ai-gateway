package translator

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/tianyuansun/ai-gateway/pkg/session"
	"github.com/tianyuansun/ai-gateway/pkg/shared"
)

// ChatToAnth translates Chat Completions API requests to Anthropic Messages API.
// Used as fallback when upstream provider has only an Anthropic endpoint.
type ChatToAnth struct{}

func (t *ChatToAnth) TranslateRequest(_ context.Context, req *Request, s *session.Session) (*UpstreamRequest, error) {
	var chatReq ChatRequest
	if err := json.Unmarshal(req.Body, &chatReq); err != nil {
		return nil, err
	}

	anthReq := &AnthropicRequest{
		Model:     req.Model,
		MaxTokens: 32768,
	}

	messages := []AnthropicMessage{}
	for _, msg := range chatReq.Messages {
		switch msg.Role {
		case "system", "developer":
			anthReq.System = msg.Content
		case "assistant":
			var content []AnthropicContent
			if len(msg.ToolCalls) > 0 {
				for _, tc := range msg.ToolCalls {
					content = append(content, AnthropicContent{
						Type:  "tool_use",
						ID:    tc.ID,
						Name:  tc.Function.Name,
						Input: parseJSON(tc.Function.Arguments),
					})
				}
			} else if msg.Content != "" {
				content = append(content, AnthropicContent{Type: "text", Text: msg.Content})
			}
			messages = append(messages, AnthropicMessage{Role: "assistant", Content: content})
		case "tool":
			messages = append(messages, AnthropicMessage{
				Role: "user",
				Content: []AnthropicContent{{
					Type:      "tool_result",
					ToolUseID: msg.ToolCallID,
					Content:   msg.Content,
				}},
			})
		default:
			messages = append(messages, AnthropicMessage{
				Role:    msg.Role,
				Content: []AnthropicContent{{Type: "text", Text: msg.Content}},
			})
		}
	}
	anthReq.Messages = messages

	if len(chatReq.Tools) > 0 {
		anthReq.Tools = make([]AnthropicTool, len(chatReq.Tools))
		for i, tool := range chatReq.Tools {
			anthReq.Tools[i] = AnthropicTool{
				Name:        tool.Name,
				Description: tool.Description,
				InputSchema: tool.Parameters,
			}
		}
	}
	if chatReq.ReasoningEffort == "high" {
		anthReq.Thinking = &ThinkingConfig{Type: "enabled", BudgetTokens: 16000}
	}

	anthBody, _ := json.Marshal(anthReq)
	return &UpstreamRequest{
		Method: "POST",
		URL:    "/messages",
		Body:   anthBody,
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

func (t *ChatToAnth) TranslateResponse(_ context.Context, upstream *http.Response, _ *Request, _ *session.Session) (*Response, error) {
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

	msg := ChatMessage{Role: "assistant"}
	for _, c := range anthResp.Content {
		switch c.Type {
		case "text":
			msg.Content = c.Text
		case "tool_use":
			args, _ := json.Marshal(c.Input)
			msg.ToolCalls = append(msg.ToolCalls, ChatToolCall{
				ID:       c.ID,
				Type:     "function",
				Function: session.FunctionCall{Name: c.Name, Arguments: string(args)},
			})
		}
	}

	chatResp := &ChatResponse{
		ID:     anthResp.ID,
		Object: "chat.completion",
		Choices: []ChatChoice{{Index: 0, Message: msg}},
		Usage: &Usage{
			PromptTokens:     anthResp.Usage.InputTokens,
			CompletionTokens: anthResp.Usage.OutputTokens,
			TotalTokens:      anthResp.Usage.InputTokens + anthResp.Usage.OutputTokens,
		},
	}
	respBody, _ := json.Marshal(chatResp)
	return &Response{StatusCode: 200, Body: respBody, ReasoningContent: reasoningContent}, nil
}

func (t *ChatToAnth) UpdateSession(s *session.Session, _ *Request, resp *Response) {
	if resp.ReasoningContent != "" {
		s.ReasoningRecords = append(s.ReasoningRecords, session.Reasoning{
			Content: resp.ReasoningContent,
		})
	}
}
