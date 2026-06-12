package translator

import (
	"context"
	"encoding/json"
	"io"

	"github.com/tianyuansun/ai-gateway/pkg/session"
)

// ResToAnth translates OpenAI Responses API requests to Anthropic Messages API.
type ResToAnth struct{}

func (t *ResToAnth) TranslateRequest(_ context.Context, req *Request, s *session.Session) (*UpstreamRequest, error) {
	var body ResponsesRequest
	if err := json.Unmarshal(req.Body, &body); err != nil {
		return nil, err
	}

	anthReq := t.buildMessages(s, &body)
	anthReq.Model = req.Model

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

func (t *ResToAnth) buildMessages(s *session.Session, body *ResponsesRequest) *AnthropicRequest {
	req := &AnthropicRequest{
		MaxTokens: 32768,
	}

	if body.Reasoning != nil && body.Reasoning.Effort == "high" {
		req.Thinking = &ThinkingConfig{Type: "enabled", BudgetTokens: 16000}
	}

	for _, item := range body.Input {
		switch item.Type {
		case "message":
			if item.Role == "system" || item.Role == "developer" {
				req.System = item.extractText()
			} else {
				req.Messages = append(req.Messages, AnthropicMessage{
					Role:    item.Role,
					Content: []AnthropicContent{{Type: "text", Text: item.extractText()}},
				})
			}
		case "function_call":
			req.Messages = append(req.Messages, AnthropicMessage{
				Role: "assistant",
				Content: []AnthropicContent{{
					Type:  "tool_use",
					ID:    item.CallID,
					Name:  item.Name,
					Input: parseJSON(item.Arguments),
				}},
			})
		case "function_call_output":
			req.Messages = append(req.Messages, AnthropicMessage{
				Role: "user",
				Content: []AnthropicContent{{
					Type:        "tool_result",
					ToolUseID:   item.CallID,
					Content:     item.Output,
				}},
			})
		}
	}

	if len(body.Tools) > 0 {
		req.Tools = make([]AnthropicTool, len(body.Tools))
		for i, tool := range body.Tools {
			req.Tools[i] = AnthropicTool{
				Name:        tool.Name,
				Description: tool.Description,
				InputSchema: tool.Parameters,
			}
		}
	}

	return req
}

func parseJSON(s string) any {
	var v any
	json.Unmarshal([]byte(s), &v)
	return v
}

func (t *ResToAnth) TranslateStream(_ context.Context, upstream io.Reader, _ *Request, _ *session.Session) <-chan SSEEvent {
	ch := make(chan SSEEvent)
	go func() {
		defer close(ch)
		data, _ := io.ReadAll(upstream)
		ch <- SSEEvent{Data: data}
	}()
	return ch
}

func (t *ResToAnth) TranslateResponse(_ context.Context, upstreamBody []byte, _ *Request, _ *session.Session) (*Response, error) {
	var anthResp AnthropicResponse
	if err := json.Unmarshal(upstreamBody, &anthResp); err != nil {
		return nil, err
	}

	resp := t.convertToResponse(&anthResp)

	respBody, _ := json.Marshal(resp)
	return &Response{StatusCode: 200, Body: respBody}, nil
}

func (t *ResToAnth) convertToResponse(anthResp *AnthropicResponse) *ResponsesResponse {
	output := []OutputItem{}

	for _, c := range anthResp.Content {
		switch c.Type {
		case "text":
			output = append(output, OutputItem{
				Type: "message",
				Role: "assistant",
				Content: []ContentPart{{Type: "output_text", Text: c.Text}},
			})
		case "tool_use":
			args, _ := json.Marshal(c.Input)
			output = append(output, OutputItem{
				Type:      "function_call",
				CallID:    c.ID,
				Name:      c.Name,
				Arguments: string(args),
			})
		}
	}

	return &ResponsesResponse{
		ID:     anthResp.ID,
		Object: "response",
		Output: output,
		Usage:  t.convertUsage(anthResp.Usage),
	}
}

func (t *ResToAnth) convertUsage(u *AnthropicUsage) *Usage {
	if u == nil {
		return nil
	}
	return &Usage{
		PromptTokens:     u.InputTokens,
		CompletionTokens: u.OutputTokens,
		TotalTokens:      u.InputTokens + u.OutputTokens,
	}
}

func (t *ResToAnth) UpdateSession(_ *session.Session, _ *Request, _ *Response) {}

// --- Anthropic types ---

type AnthropicRequest struct {
	Model     string             `json:"model"`
	System    string             `json:"system,omitempty"`
	Messages  []AnthropicMessage `json:"messages"`
	Tools     []AnthropicTool    `json:"tools,omitempty"`
	MaxTokens int                `json:"max_tokens"`
	Thinking  *ThinkingConfig    `json:"thinking,omitempty"`
}

type AnthropicMessage struct {
	Role    string             `json:"role"`
	Content []AnthropicContent `json:"content"`
}

type AnthropicContent struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Input     any    `json:"input,omitempty"`
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
}

type AnthropicTool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"input_schema"`
}

type ThinkingConfig struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens"`
}

type AnthropicResponse struct {
	ID      string             `json:"id"`
	Type    string             `json:"type"`
	Role    string             `json:"role"`
	Content []AnthropicContent `json:"content"`
	Usage   *AnthropicUsage    `json:"usage,omitempty"`
}

type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}
