// Package chat provides spec-accurate Go types for the OpenAI Chat Completions API.
// Types are derived from the official OpenAI OpenAPI specification at:
// https://github.com/openai/openai-openapi/blob/master/openapi.yaml
package chat

import "encoding/json"

// ---------------------------------------------------------------------------
// Request types
// ---------------------------------------------------------------------------

// ChatCompletionRequest is the request body for POST /v1/chat/completions.
type ChatCompletionRequest struct {
	Model               string                           `json:"model"`
	Messages            []ChatCompletionMessage          `json:"messages"`
	Temperature         *float64                         `json:"temperature,omitempty"`
	TopP                *float64                         `json:"top_p,omitempty"`
	N                   *int                             `json:"n,omitempty"`
	Stream              bool                             `json:"stream,omitempty"`
	Stop                *ChatCompletionStop              `json:"stop,omitempty"`
	MaxTokens           *int                             `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int                             `json:"max_completion_tokens,omitempty"`
	PresencePenalty     *float64                         `json:"presence_penalty,omitempty"`
	FrequencyPenalty    *float64                         `json:"frequency_penalty,omitempty"`
	LogitBias           map[string]int                   `json:"logit_bias,omitempty"`
	User                string                           `json:"user,omitempty"`
	Tools               []ChatCompletionTool             `json:"tools,omitempty"`
	ToolChoice          *ChatCompletionToolChoice        `json:"tool_choice,omitempty"`
	ResponseFormat      *ChatCompletionResponseFormat    `json:"response_format,omitempty"`
	Seed                *int                             `json:"seed,omitempty"`
	StreamOptions       *ChatCompletionStreamOptions     `json:"stream_options,omitempty"`
	ParallelToolCalls   *bool                            `json:"parallel_tool_calls,omitempty"`
	ReasoningEffort     *string                          `json:"reasoning_effort,omitempty"`
	Logprobs            *bool                            `json:"logprobs,omitempty"`
	TopLogprobs         *int                             `json:"top_logprobs,omitempty"`
}

// ChatCompletionStop is a union: a single stop string or a list of stop strings.
type ChatCompletionStop struct {
	String *string  `json:"-"`
	Array  []string `json:"-"`
}

func (s ChatCompletionStop) MarshalJSON() ([]byte, error) {
	if s.String != nil {
		return json.Marshal(*s.String)
	}
	return json.Marshal(s.Array)
}

func (s *ChatCompletionStop) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var str string
		if err := json.Unmarshal(data, &str); err != nil {
			return err
		}
		s.String = &str
		return nil
	}
	return json.Unmarshal(data, &s.Array)
}

// ChatCompletionToolChoice is a union: a string ("none", "auto", "required")
// or a ChatCompletionNamedToolChoice object.
type ChatCompletionToolChoice struct {
	String *string                        `json:"-"`
	Object *ChatCompletionNamedToolChoice `json:"-"`
}

func (c ChatCompletionToolChoice) MarshalJSON() ([]byte, error) {
	if c.String != nil {
		return json.Marshal(*c.String)
	}
	if c.Object != nil {
		return json.Marshal(c.Object)
	}
	return json.Marshal(nil)
}

func (c *ChatCompletionToolChoice) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var str string
		if err := json.Unmarshal(data, &str); err != nil {
			return err
		}
		c.String = &str
		return nil
	}
	c.Object = &ChatCompletionNamedToolChoice{}
	return json.Unmarshal(data, c.Object)
}

// ChatCompletionNamedToolChoice forces a specific tool to be called.
type ChatCompletionNamedToolChoice struct {
	Type     string `json:"type"`
	Function struct {
		Name string `json:"name"`
	} `json:"function"`
}

// ChatCompletionStreamOptions configures streaming behaviour.
type ChatCompletionStreamOptions struct {
	IncludeUsage       *bool `json:"include_usage,omitempty"`
	IncludeObfuscation *bool `json:"include_obfuscation,omitempty"`
}

// ChatCompletionResponseFormat controls the output format.
type ChatCompletionResponseFormat struct {
	Type       string                                      `json:"type"`
	JSONSchema *ChatCompletionResponseFormatJSONSchema     `json:"json_schema,omitempty"`
}

// ChatCompletionResponseFormatJSONSchema defines a structured output schema.
type ChatCompletionResponseFormatJSONSchema struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Schema      json.RawMessage `json:"schema,omitempty"`
	Strict      *bool           `json:"strict,omitempty"`
}

// ---------------------------------------------------------------------------
// Message types
// ---------------------------------------------------------------------------

// ChatCompletionMessage represents a message in the conversation.
// Used for both request messages and response message objects.
type ChatCompletionMessage struct {
	Role         string                          `json:"role"`
	Content      *ChatCompletionMessageContent   `json:"content"`
	Refusal      *string                         `json:"refusal,omitempty"`
	ToolCalls    []ChatCompletionMessageToolCall `json:"tool_calls,omitempty"`
	Name         string                          `json:"name,omitempty"`
	ToolCallID   string                          `json:"tool_call_id,omitempty"`
	FunctionCall *ChatCompletionFunctionCall     `json:"function_call,omitempty"`
}

// ChatCompletionStreamDelta is the delta in a streaming chunk choice.
// All fields are optional — only changed fields are populated.
type ChatCompletionStreamDelta struct {
	Role         *string                                `json:"role,omitempty"`
	Content      *ChatCompletionMessageContent          `json:"content,omitempty"`
	ToolCalls    []ChatCompletionMessageToolCallChunk   `json:"tool_calls,omitempty"`
	Refusal      *string                                `json:"refusal,omitempty"`
	FunctionCall *ChatCompletionFunctionCall            `json:"function_call,omitempty"`
}

// ChatCompletionFunctionCall is the legacy function call (deprecated, use tool_calls).
type ChatCompletionFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ---------------------------------------------------------------------------
// Content union types
// ---------------------------------------------------------------------------

// ChatCompletionMessageContent is a union: either a plain string or an array
// of ChatCompletionContentPart (for multimodal messages).
type ChatCompletionMessageContent struct {
	String *string                      `json:"-"`
	Parts  []ChatCompletionContentPart  `json:"-"`
}

func (c ChatCompletionMessageContent) MarshalJSON() ([]byte, error) {
	if c.String != nil {
		return json.Marshal(*c.String)
	}
	if c.Parts != nil {
		return json.Marshal(c.Parts)
	}
	return json.Marshal(nil)
}

func (c *ChatCompletionMessageContent) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		c.String = &s
		return nil
	}
	return json.Unmarshal(data, &c.Parts)
}

// ChatCompletionContentPart is one part of a multimodal message content array.
type ChatCompletionContentPart struct {
	Type       string                       `json:"type"`
	Text       string                       `json:"text,omitempty"`
	ImageURL   *ChatCompletionImageURL      `json:"image_url,omitempty"`
	InputAudio *ChatCompletionInputAudio    `json:"input_audio,omitempty"`
	File       *ChatCompletionFile          `json:"file,omitempty"`
	Refusal    *string                      `json:"refusal,omitempty"`
}

// ChatCompletionImageURL is an image reference in a content part.
type ChatCompletionImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

// ChatCompletionInputAudio is an audio input in a content part.
type ChatCompletionInputAudio struct {
	Data   string `json:"data"`
	Format string `json:"format"`
}

// ChatCompletionFile is a file reference in a content part.
type ChatCompletionFile struct {
	FileID   string `json:"file_id,omitempty"`
	FileData string `json:"file_data,omitempty"`
	Filename string `json:"filename,omitempty"`
}

// ---------------------------------------------------------------------------
// Tool types
// ---------------------------------------------------------------------------

// ChatCompletionTool defines a tool available to the model.
type ChatCompletionTool struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

// FunctionDefinition is the JSON Schema definition for a function tool.
type FunctionDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
	Strict      *bool           `json:"strict,omitempty"`
}

// ChatCompletionMessageToolCall is a tool call inside an assistant response message.
type ChatCompletionMessageToolCall struct {
	ID       string                         `json:"id"`
	Type     string                         `json:"type"`
	Function ChatCompletionToolCallFunction `json:"function"`
}

// ChatCompletionMessageToolCallChunk is an incremental tool call in a streaming delta.
// It includes an explicit Index field so that concurrent tool calls can be
// reassembled correctly.
type ChatCompletionMessageToolCallChunk struct {
	Index    int                              `json:"index"`
	ID       string                           `json:"id,omitempty"`
	Type     string                           `json:"type,omitempty"`
	Function *ChatCompletionToolCallFunction  `json:"function,omitempty"`
}

// ChatCompletionToolCallFunction is the function invocation details within a tool call.
type ChatCompletionToolCallFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments"`
}

// ---------------------------------------------------------------------------
// Response types
// ---------------------------------------------------------------------------

// ChatCompletion is the response body from POST /v1/chat/completions.
type ChatCompletion struct {
	ID                string                   `json:"id"`
	Object            string                   `json:"object"`
	Created           int64                    `json:"created"`
	Model             string                   `json:"model"`
	Choices           []ChatCompletionChoice   `json:"choices"`
	Usage             *CompletionUsage         `json:"usage,omitempty"`
	SystemFingerprint string                   `json:"system_fingerprint,omitempty"`
}

// ChatCompletionChoice is one completion choice in the response.
type ChatCompletionChoice struct {
	Index        int                      `json:"index"`
	Message      ChatCompletionMessage    `json:"message"`
	FinishReason *string                  `json:"finish_reason,omitempty"`
	Logprobs     *ChatCompletionLogprobs  `json:"logprobs,omitempty"`
}

// ChatCompletionLogprobs are log-probability details for a choice.
type ChatCompletionLogprobs struct {
	Content []ChatCompletionTokenLogprob `json:"content,omitempty"`
	Refusal []ChatCompletionTokenLogprob `json:"refusal,omitempty"`
}

// ChatCompletionTokenLogprob is a single token's log-probability info.
type ChatCompletionTokenLogprob struct {
	Token       string    `json:"token"`
	Logprob     float64   `json:"logprob"`
	Bytes       []int     `json:"bytes,omitempty"`
	TopLogprobs []struct {
		Token   string  `json:"token"`
		Logprob float64 `json:"logprob"`
		Bytes   []int   `json:"bytes,omitempty"`
	} `json:"top_logprobs,omitempty"`
}

// CompletionUsage contains token usage statistics.
type CompletionUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ---------------------------------------------------------------------------
// Streaming types
// ---------------------------------------------------------------------------

// ChatCompletionChunk is a server-sent event for a streaming completion.
type ChatCompletionChunk struct {
	ID                string                        `json:"id"`
	Object            string                        `json:"object"`
	Created           int64                         `json:"created"`
	Model             string                        `json:"model"`
	Choices           []ChatCompletionChunkChoice   `json:"choices"`
	Usage             *CompletionUsage              `json:"usage,omitempty"`
	SystemFingerprint string                        `json:"system_fingerprint,omitempty"`
}

// ChatCompletionChunkChoice is one choice in a streaming chunk.
type ChatCompletionChunkChoice struct {
	Index        int                         `json:"index"`
	Delta        ChatCompletionStreamDelta   `json:"delta"`
	FinishReason *string                     `json:"finish_reason,omitempty"`
	Logprobs     *ChatCompletionLogprobs     `json:"logprobs,omitempty"`
}
