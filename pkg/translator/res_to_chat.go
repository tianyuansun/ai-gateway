package translator

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/tianyuansun/ai-gateway/pkg/session"
	"github.com/tianyuansun/ai-gateway/pkg/shared"
)

// ResToChat translates OpenAI Responses API requests to Chat Completions API.
type ResToChat struct{}

func (t *ResToChat) TranslateRequest(_ context.Context, req *Request, s *session.Session) (*UpstreamRequest, error) {
	var body ResponsesRequest
	if err := json.Unmarshal(req.Body, &body); err != nil {
		return nil, err
	}

	// Rebuild full message history from session
	messages := t.rebuildMessages(s, &body)

	chatReq := ChatRequest{
		Model:    req.Model,
		Messages: messages,
		Stream:   true,
	}
	if len(body.Tools) > 0 {
		chatReq.Tools = body.Tools
	}
	if body.Reasoning != nil {
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

func (t *ResToChat) rebuildMessages(s *session.Session, body *ResponsesRequest) []ChatMessage {
	if s != nil && len(s.Messages) > 0 {
		return t.sessionMessages(s)
	}

	msgs := make([]ChatMessage, 0, len(body.Input))
	for _, item := range body.Input {
		switch item.Type {
		case "message":
			msgs = append(msgs, ChatMessage{
				Role:    item.Role,
				Content: item.extractText(),
			})
		case "function_call":
			msgs = append(msgs, ChatMessage{
				Role: "assistant",
				ToolCalls: []ChatToolCall{{
					ID:       item.CallID,
					Type:     "function",
					Function: item.Function,
				}},
			})
		case "function_call_output":
			msgs = append(msgs, ChatMessage{
				Role:       "tool",
				ToolCallID: item.CallID,
				Content:    item.Output,
			})
		}
	}
	return msgs
}

func (t *ResToChat) sessionMessages(s *session.Session) []ChatMessage {
	msgs := make([]ChatMessage, len(s.Messages))
	for i, m := range s.Messages {
		msgs[i] = ChatMessage{
			Role:       m.Role,
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
			Name:       m.Name,
		}
		if len(m.ToolCalls) > 0 {
			msgs[i].ToolCalls = make([]ChatToolCall, len(m.ToolCalls))
			for j, tc := range m.ToolCalls {
				msgs[i].ToolCalls[j] = ChatToolCall{
					ID:       tc.ID,
					Type:     tc.Type,
					Function: tc.Function,
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
		started := false
		completed := false
		itemStarted := false
		var responseID string
		for sseEv := range shared.ParseSSE(upstream) {
			if sseEv.Data == "[DONE]" {
				if itemStarted {
					itemDone, _ := json.Marshal(map[string]any{
						"type":         "response.output_item.done",
						"output_index": 0,
						"item": map[string]any{
							"id": responseID + "_item", "type": "message", "role": "assistant", "status": "in_progress",
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
				startData, _ := json.Marshal(map[string]any{
					"type": "response.created",
					"response": map[string]any{
						"id": responseID, "object": "response",
					},
				})
				ch <- SSEEvent{Event: "response.created", Data: startData}
			}

			for _, choice := range chunk.Choices {
				if choice.Delta.Content != "" && !itemStarted {
					itemStarted = true
					itemData, _ := json.Marshal(map[string]any{
						"type":         "response.output_item.added",
						"output_index": 0,
						"item": map[string]any{
							"id": responseID + "_item", "type": "message", "role": "assistant", "status": "in_progress",
						},
					})
					ch <- SSEEvent{Event: "response.output_item.added", Data: itemData}
					partData, _ := json.Marshal(map[string]any{
						"type":          "response.content_part.added",
						"item_id":       responseID + "_item",
						"output_index":  0,
						"content_index": 0,
						"part":          map[string]any{"type": "output_text", "text": ""},
					})
					ch <- SSEEvent{Event: "response.content_part.added", Data: partData}
				}
				if choice.Delta.Content != "" {
					deltaData, _ := json.Marshal(map[string]any{
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
						itemDone, _ := json.Marshal(map[string]any{
							"type":         "response.output_item.done",
							"output_index": 0,
							"item": map[string]any{
								"id": responseID + "_item", "type": "message", "role": "assistant", "status": "in_progress",
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
		}
		// Final completed event if not already sent.
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

func (t *ResToChat) TranslateResponse(_ context.Context, upstream *http.Response, _ *Request, s *session.Session) (*Response, error) {
	body, err := io.ReadAll(upstream.Body)
	if err != nil {
		return nil, err
	}
	upstream.Body.Close()

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, err
	}

	// Extract reasoning_content from the raw JSON (not in ChatResponse struct).
	reasoningContent := extractReasoningContent(body)

	resp := t.convertToResponse(&chatResp)

	if s != nil {
		t.appendToSession(s, &chatResp)
	}

	respBody, _ := json.Marshal(resp)
	return &Response{StatusCode: 200, Body: respBody, ReasoningContent: reasoningContent}, nil
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

func (t *ResToChat) convertToResponse(chatResp *ChatResponse) *ResponsesResponse {
	msg := chatResp.Choices[0].Message

	output := []OutputItem{}
	if msg.Content != "" {
		output = append(output, OutputItem{
			Type: "message",
			Role: "assistant",
			Content: []ContentPart{{Type: "output_text", Text: msg.Content}},
		})
	}
	for _, tc := range msg.ToolCalls {
		args, _ := json.Marshal(tc.Function.Arguments)
		output = append(output, OutputItem{
			Type:      "function_call",
			CallID:    tc.ID,
			Name:      tc.Function.Name,
			Arguments: string(args),
		})
	}

	return &ResponsesResponse{
		ID:     chatResp.ID,
		Object: "response",
		Output: output,
		Usage:  chatResp.Usage,
	}
}

func (t *ResToChat) appendToSession(s *session.Session, chatResp *ChatResponse) {
	msg := chatResp.Choices[0].Message
	s.Messages = append(s.Messages, session.Message{
		Role:    "assistant",
		Content: msg.Content,
	})
	if len(msg.ToolCalls) > 0 {
		last := &s.Messages[len(s.Messages)-1]
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

func (t *ResToChat) UpdateSession(s *session.Session, _ *Request, resp *Response) {
	if resp.ReasoningContent != "" {
		s.ReasoningRecords = append(s.ReasoningRecords, session.Reasoning{
			Content: resp.ReasoningContent,
		})
	}
}

// --- Request/Response types ---

type ResponsesRequest struct {
	Model               string        `json:"model"`
	Input               []InputItem   `json:"input"`
	Tools               []Tool        `json:"tools,omitempty"`
	PreviousResponseID  string        `json:"previous_response_id,omitempty"`
	Reasoning           *Reasoning    `json:"reasoning,omitempty"`
}

type InputItem struct {
	Type      string        `json:"type"`
	Role      string        `json:"role,omitempty"`
	Content   []ContentPart `json:"content,omitempty"`
	CallID    string        `json:"call_id,omitempty"`
	Name      string        `json:"name,omitempty"`
	Arguments string        `json:"arguments,omitempty"`
	Output    string        `json:"output,omitempty"`
	Function  session.FunctionCall `json:"function,omitempty"`
}

func (item InputItem) extractText() string {
	for _, c := range item.Content {
		if c.Text != "" {
			return c.Text
		}
	}
	return ""
}

type ContentPart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type Tool struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters any    `json:"parameters,omitempty"`
}

type Reasoning struct {
	Effort string `json:"effort"`
}

type ChatRequest struct {
	Model           string        `json:"model"`
	Messages        []ChatMessage `json:"messages"`
	Tools           []Tool        `json:"tools,omitempty"`
	ReasoningEffort string        `json:"reasoning_effort,omitempty"`
	Stream          bool          `json:"stream,omitempty"`
}

type ChatMessage struct {
	Role             string          `json:"role"`
	Content          string          `json:"content,omitempty"`
	ReasoningContent string          `json:"reasoning_content,omitempty"`
	ToolCalls        []ChatToolCall  `json:"tool_calls,omitempty"`
	ToolCallID       string          `json:"tool_call_id,omitempty"`
	Name             string          `json:"name,omitempty"`
}

type ChatToolCall struct {
	ID       string              `json:"id"`
	Type     string              `json:"type"`
	Function session.FunctionCall `json:"function"`
}

type ChatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Choices []ChatChoice `json:"choices"`
	Usage   *Usage       `json:"usage,omitempty"`
}

type ChatChoice struct {
	Index   int         `json:"index"`
	Message ChatMessage `json:"message"`
}

type ResponsesResponse struct {
	ID     string       `json:"id"`
	Object string       `json:"object"`
	Output []OutputItem `json:"output"`
	Usage  *Usage       `json:"usage,omitempty"`
}

type OutputItem struct {
	Type      string        `json:"type"`
	Role      string        `json:"role,omitempty"`
	Content   []ContentPart `json:"content,omitempty"`
	CallID    string        `json:"call_id,omitempty"`
	Name      string        `json:"name,omitempty"`
	Arguments string        `json:"arguments,omitempty"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
