// Package anthropic provides spec-accurate Go types for the Anthropic Messages API.
// Types are derived from the official JSON Schema at:
// https://github.com/api-evangelist/anthropic/blob/main/json-schema/anthropic-message-schema.json
package anthropic

import "encoding/json"

// MessageRequest is the request body for POST /v1/messages.
type MessageRequest struct {
	Model        string           `json:"model"`
	Messages     []MessageParam   `json:"messages"`
	MaxTokens    int              `json:"max_tokens"`
	System       *SystemContent   `json:"system,omitempty"`
	Temperature  *float64         `json:"temperature,omitempty"`
	TopP         *float64         `json:"top_p,omitempty"`
	TopK         *int             `json:"top_k,omitempty"`
	Stream       bool             `json:"stream,omitempty"`
	StopSequences []string        `json:"stop_sequences,omitempty"`
	Tools        []ToolDefinition `json:"tools,omitempty"`
	ToolChoice   *ToolChoice      `json:"tool_choice,omitempty"`
	Thinking     *ThinkingConfig  `json:"thinking,omitempty"`
	Metadata     *Metadata        `json:"metadata,omitempty"`
}

// SystemContent is a union: either a string or an array of TextBlockParam.
type SystemContent struct {
	String *string          `json:"-"`
	Blocks []TextBlockParam `json:"-"`
}

func (s SystemContent) MarshalJSON() ([]byte, error) {
	if s.String != nil {
		return json.Marshal(*s.String)
	}
	return json.Marshal(s.Blocks)
}

func (s *SystemContent) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var str string
		if err := json.Unmarshal(data, &str); err != nil {
			return err
		}
		s.String = &str
		return nil
	}
	return json.Unmarshal(data, &s.Blocks)
}

// MessageParam is a message in the conversation.
type MessageParam struct {
	Role    string              `json:"role"`
	Content []ContentBlockParam `json:"content"`
}

// ContentBlockParam is a content block in a request message.
type ContentBlockParam struct {
	Type         string           `json:"type"`
	Text         string           `json:"text,omitempty"`
	Thinking     string           `json:"thinking,omitempty"`
	Signature    string           `json:"signature,omitempty"`
	Source       *ImageSource     `json:"source,omitempty"`
	ID           string           `json:"id,omitempty"`
	Name         string           `json:"name,omitempty"`
	Input        json.RawMessage  `json:"input,omitempty"`
	ToolUseID    string           `json:"tool_use_id,omitempty"`
	Content      string           `json:"content,omitempty"`
	CacheControl *CacheControl    `json:"cache_control,omitempty"`
}

// TextBlockParam is a text-only content block.
type TextBlockParam struct {
	Type         string        `json:"type"`
	Text         string        `json:"text"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

// ImageBlockParam is an image content block.
type ImageBlockParam struct {
	Type   string       `json:"type"`
	Source ImageSource  `json:"source"`
}

// ImageSource identifies the source of an image.
type ImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
	URL       string `json:"url,omitempty"`
}

// DocumentBlockParam is a document content block.
type DocumentBlockParam struct {
	Type   string         `json:"type"`
	Source DocumentSource `json:"source"`
}

// DocumentSource identifies the source of a document.
type DocumentSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
	URL       string `json:"url,omitempty"`
}

// ToolUseBlockParam is a tool use content block in a request.
type ToolUseBlockParam struct {
	Type  string          `json:"type"`
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// ToolResultBlockParam is a tool result content block.
type ToolResultBlockParam struct {
	Type      string `json:"type"`
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	IsError   *bool  `json:"is_error,omitempty"`
}

// MessageResponse is the response from POST /v1/messages.
type MessageResponse struct {
	ID         string                  `json:"id"`
	Type       string                  `json:"type"`
	Role       string                  `json:"role"`
	Model      string                  `json:"model"`
	Content    []ResponseContentBlock  `json:"content"`
	StopReason *string                 `json:"stop_reason,omitempty"`
	StopSequence *string               `json:"stop_sequence,omitempty"`
	Usage      Usage                   `json:"usage"`
}

// ResponseContentBlock is a content block in a response.
type ResponseContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	Thinking  string          `json:"thinking,omitempty"`
	Signature string          `json:"signature,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
	Index     *int            `json:"index,omitempty"`
	Citation  *Citation       `json:"citation,omitempty"`
}

// TextBlock is a text response block.
type TextBlock struct {
	Type     string    `json:"type"`
	Text     string    `json:"text"`
	Citation *Citation `json:"citation,omitempty"`
}

// ThinkingBlock is a thinking/reasoning response block.
type ThinkingBlock struct {
	Type      string `json:"type"`
	Thinking  string `json:"thinking"`
	Signature string `json:"signature"`
}

// ToolUseBlock is a tool use response block.
type ToolUseBlock struct {
	Type  string          `json:"type"`
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// ToolDefinition defines a tool available to the model.
type ToolDefinition struct {
	Name         string          `json:"name"`
	Description  string          `json:"description,omitempty"`
	InputSchema  json.RawMessage `json:"input_schema"`
	Type         string          `json:"type,omitempty"`
	CacheControl *CacheControl   `json:"cache_control,omitempty"`
}

// ToolChoice controls tool selection.
type ToolChoice struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

// ThinkingConfig controls extended thinking.
type ThinkingConfig struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens"`
}

// Usage contains token usage information.
type Usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// CacheControl controls prompt caching.
type CacheControl struct {
	Type string `json:"type"`
}

// Citation is a citation in a text block.
type Citation struct {
	Type      string `json:"type"`
	StartIdx  int    `json:"start_idx,omitempty"`
	EndIdx    int    `json:"end_idx,omitempty"`
	Text      string `json:"text,omitempty"`
	URL       string `json:"url,omitempty"`
	Title     string `json:"title,omitempty"`
}

// Metadata contains request metadata (user ID).
type Metadata struct {
	UserID string `json:"user_id,omitempty"`
}

// ErrorResponse is the API error response.
type ErrorResponse struct {
	Type    string `json:"type"`
	Error   struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}
