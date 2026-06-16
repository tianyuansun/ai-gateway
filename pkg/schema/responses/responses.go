// Package responses provides spec-accurate Go types for the OpenAI Responses API.
// Types are derived from the official OpenAI API reference at:
// https://platform.openai.com/docs/api-reference/responses
package responses

import "encoding/json"

// =============================================================================
// Request Types
// =============================================================================

// ResponseRequest represents a request to POST /v1/responses.
type ResponseRequest struct {
	// Model is the model ID to use (required).
	Model string `json:"model"`
	// Input is the input to the model: either a plain string or an array of items.
	Input ResponseInput `json:"input"`
	// Instructions is a developer message that instructs the model.
	Instructions *string `json:"instructions,omitempty"`
	// MaxOutputTokens is an upper bound on generated tokens.
	MaxOutputTokens *int `json:"max_output_tokens,omitempty"`
	// Temperature controls randomness (0–2). Not supported by GPT-5 models.
	Temperature *float64 `json:"temperature,omitempty"`
	// TopP is nucleus sampling parameter (0–1).
	TopP *float64 `json:"top_p,omitempty"`
	// Stream enables SSE streaming of the response.
	Stream bool `json:"stream,omitempty"`
	// PreviousResponseID chains this request to a previous response for multi-turn.
	PreviousResponseID *string `json:"previous_response_id,omitempty"`
	// Tools available to the model.
	Tools []ToolDefinition `json:"tools,omitempty"`
	// ToolChoice controls tool selection: "auto", "none", "required", or a specific tool.
	ToolChoice json.RawMessage `json:"tool_choice,omitempty"`
	// ParallelToolCalls enables multiple tool calls per response.
	ParallelToolCalls *bool `json:"parallel_tool_calls,omitempty"`
	// Reasoning configures reasoning effort for reasoning models.
	Reasoning *ReasoningConfig `json:"reasoning,omitempty"`
	// Text configures text output format (plain, json_object, json_schema).
	Text *ResponseTextConfig `json:"text,omitempty"`
	// Truncation controls context overflow behaviour: "auto" or "disabled".
	Truncation *string `json:"truncation,omitempty"`
	// Store persists the response in OpenAI's storage.
	Store *bool `json:"store,omitempty"`
	// Metadata is up to 16 key-value pairs of user metadata.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ResponseInput is a union: either a plain string or an array of ResponseInputItem.
// It follows the same pattern as SystemContent in the Anthropic schema.
type ResponseInput struct {
	String *string             `json:"-"`
	Items  []ResponseInputItem `json:"-"`
}

// MarshalJSON marshals ResponseInput as either a string or an array.
func (ri ResponseInput) MarshalJSON() ([]byte, error) {
	if ri.String != nil {
		return json.Marshal(*ri.String)
	}
	if ri.Items != nil {
		return json.Marshal(ri.Items)
	}
	return json.Marshal("")
}

// UnmarshalJSON unmarshals ResponseInput from either a string or an array.
func (ri *ResponseInput) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	if data[0] == '"' {
		var str string
		if err := json.Unmarshal(data, &str); err != nil {
			return err
		}
		ri.String = &str
		return nil
	}
	return json.Unmarshal(data, &ri.Items)
}

// ResponseInputItem is a flat struct covering all input item variants.
// Uses a type discriminator field. Variants: message, function_call, function_call_output, item_reference.
type ResponseInputItem struct {
	// Type discriminator: "message", "function_call", "function_call_output", "item_reference".
	Type string `json:"type"`
	// Role is for message items: "user", "system", "developer", "assistant".
	Role string `json:"role,omitempty"`
	// Content is for message items. Can be a string or an array of content parts.
	Content json.RawMessage `json:"content,omitempty"`
	// CallID is for function_call and function_call_output items.
	CallID string `json:"call_id,omitempty"`
	// Name is for function_call items.
	Name string `json:"name,omitempty"`
	// Arguments is a JSON string of arguments for function_call items.
	Arguments string `json:"arguments,omitempty"`
	// Output is a JSON string of the function output for function_call_output items.
	Output string `json:"output,omitempty"`
	// ID is for function_call items (carried from a prior response) and item_reference.
	ID string `json:"id,omitempty"`
	// Status is for message items that originate from a prior response output.
	Status string `json:"status,omitempty"`
	// Summary is for reasoning items: array of summary_text blocks containing the visible reasoning.
	Summary json.RawMessage `json:"summary,omitempty"`
	// EncryptedContent is for server-encrypted reasoning items (opaque, passed through for passthrough).
	EncryptedContent string `json:"encrypted_content,omitempty"`
}

// =============================================================================
// Response Types
// =============================================================================

// Response is the top-level response object from POST /v1/responses.
type Response struct {
	// ID is the unique response identifier, e.g. "resp_67ccd2bed1ec81...".
	ID string `json:"id"`
	// Object is always "response".
	Object string `json:"object"`
	// CreatedAt is a Unix timestamp (seconds) of response creation.
	CreatedAt int64 `json:"created_at"`
	// Status is "completed", "in_progress", "failed", or "incomplete".
	Status string `json:"status"`
	// CompletedAt is a Unix timestamp when the response completed.
	CompletedAt *int64 `json:"completed_at,omitempty"`
	// Error is present when status is "failed".
	Error *ResponseError `json:"error,omitempty"`
	// IncompleteDetails indicates why a response was truncated.
	IncompleteDetails *ResponseIncompleteDetails `json:"incomplete_details,omitempty"`
	// Instructions echoed back from the request.
	Instructions *string `json:"instructions,omitempty"`
	// MaxOutputTokens echoed back.
	MaxOutputTokens *int `json:"max_output_tokens,omitempty"`
	// Model used for generation.
	Model string `json:"model"`
	// Output is an array of output items (messages, function calls, etc.).
	Output []ResponseOutputItem `json:"output"`
	// ParallelToolCalls indicates whether parallel tool calls were allowed.
	ParallelToolCalls bool `json:"parallel_tool_calls,omitempty"`
	// PreviousResponseID links to the prior response in a chain.
	PreviousResponseID *string `json:"previous_response_id,omitempty"`
	// Reasoning echoed back from the request.
	Reasoning *ReasoningConfig `json:"reasoning,omitempty"`
	// Store indicates whether the response was persisted.
	Store bool `json:"store,omitempty"`
	// Temperature echoed back.
	Temperature *float64 `json:"temperature,omitempty"`
	// Text echoed back.
	Text *ResponseTextConfig `json:"text,omitempty"`
	// ToolChoice echoed back.
	ToolChoice json.RawMessage `json:"tool_choice,omitempty"`
	// Tools echoed back.
	Tools []json.RawMessage `json:"tools,omitempty"`
	// TopP echoed back.
	TopP *float64 `json:"top_p,omitempty"`
	// Truncation echoed back.
	Truncation string `json:"truncation,omitempty"`
	// Usage contains token usage statistics.
	Usage *ResponseUsage `json:"usage,omitempty"`
	// Metadata echoed back.
	Metadata map[string]string `json:"metadata,omitempty"`
	// ServiceTier indicates "auto", "default", "flex", or "priority".
	ServiceTier string `json:"service_tier,omitempty"`
}

// ResponseOutputItem is a flat struct covering all output item variants.
// Uses a type discriminator. Key variants: message, function_call, reasoning, web_search_call.
type ResponseOutputItem struct {
	// Type discriminator: "message", "function_call", "reasoning", "web_search_call", etc.
	Type string `json:"type"`
	// ID is the unique identifier of the output item.
	ID string `json:"id,omitempty"`
	// Status is for message items: "in_progress", "completed", "incomplete".
	Status string `json:"status,omitempty"`
	// Role is for message items; always "assistant".
	Role string `json:"role,omitempty"`
	// Content is for message items: an array of content parts.
	Content []ResponseContentPart `json:"content,omitempty"`
	// CallID is for function_call items.
	CallID string `json:"call_id,omitempty"`
	// Name is the function name for function_call items.
	Name string `json:"name,omitempty"`
	// Arguments is a JSON string of function arguments for function_call items.
	Arguments string `json:"arguments,omitempty"`
	// Summary is for reasoning items: array of summarized reasoning blocks.
	Summary []json.RawMessage `json:"summary,omitempty"`
}

// =============================================================================
// Named / Convenience Types
// =============================================================================

// ResponseOutputMessage is an explicit type for output items with type="message".
type ResponseOutputMessage struct {
	ID      string                `json:"id"`
	Type    string                `json:"type"`
	Status  string                `json:"status,omitempty"`
	Role    string                `json:"role"`
	Content []ResponseContentPart `json:"content"`
}

// ResponseFunctionCall is an explicit type for output items with type="function_call".
type ResponseFunctionCall struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ResponseFunctionCallOutput is an explicit type for input items with type="function_call_output".
type ResponseFunctionCallOutput struct {
	Type   string `json:"type"`
	CallID string `json:"call_id"`
	Output string `json:"output"`
}

// =============================================================================
// Content Part Types
// =============================================================================

// ResponseContentPart is a flat struct covering all content part variants within
// a message's content array. Key variants: output_text, refusal.
type ResponseContentPart struct {
	// Type discriminator: "output_text", "refusal", "input_text", "input_image", "input_file".
	Type string `json:"type"`
	// Text is for output_text parts.
	Text string `json:"text,omitempty"`
	// Annotations is for output_text parts: citations, file references, etc.
	Annotations []json.RawMessage `json:"annotations,omitempty"`
	// Refusal is for refusal parts.
	Refusal string `json:"refusal,omitempty"`
}

// =============================================================================
// Configuration Types
// =============================================================================

// ResponseUsage contains token usage statistics.
type ResponseUsage struct {
	InputTokens         int                          `json:"input_tokens"`
	InputTokensDetails  *ResponseInputTokensDetails  `json:"input_tokens_details,omitempty"`
	OutputTokens        int                          `json:"output_tokens"`
	OutputTokensDetails *ResponseOutputTokensDetails `json:"output_tokens_details,omitempty"`
	TotalTokens         int                          `json:"total_tokens"`
}

// ResponseInputTokensDetails holds detail for input token consumption.
type ResponseInputTokensDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

// ResponseOutputTokensDetails holds detail for output token consumption.
type ResponseOutputTokensDetails struct {
	ReasoningTokens int `json:"reasoning_tokens"`
}

// ResponseTextConfig controls text output formatting.
// For GPT-5 models, use Verbosity; for GPT-4o and earlier, format is set via Format.
type ResponseTextConfig struct {
	// Format is a JSON object configuring output format:
	// {"type": "text"} | {"type": "json_object"} | {"type": "json_schema", "name": "...", "schema": {...}}
	Format json.RawMessage `json:"format,omitempty"`
	// Verbosity is for GPT-5 models: "low", "medium", "high".
	Verbosity *string `json:"verbosity,omitempty"`
}

// ReasoningConfig controls reasoning effort for reasoning models (o3, o4-mini, gpt-5).
type ReasoningConfig struct {
	// Effort is the reasoning effort: "minimal", "low", "medium", "high", "xhigh".
	Effort *string `json:"effort,omitempty"`
	// Summary controls summarisation: "auto", "concise", "detailed", "none".
	Summary *string `json:"summary,omitempty"`
}

// ResponseError is the error object within a failed response.
type ResponseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ResponseIncompleteDetails indicates why a response was truncated.
type ResponseIncompleteDetails struct {
	Reason string `json:"reason,omitempty"`
}

// =============================================================================
// Tool Types
// =============================================================================

// ToolDefinition describes a tool available to the model during response generation.
type ToolDefinition struct {
	// Type is the tool type: "function", "web_search_preview", "file_search", "code_interpreter".
	Type string `json:"type"`
	// Name is the function name (required for type="function").
	Name string `json:"name,omitempty"`
	// Description is a human-readable description of the function.
	Description string `json:"description,omitempty"`
	// Parameters is a JSON Schema object describing the function parameters.
	Parameters json.RawMessage `json:"parameters,omitempty"`
}

// =============================================================================
// Streaming Types
// =============================================================================

// ResponseStreamEvent represents a Server-Sent Event in the streaming protocol.
// Discriminated by the Type field. Common event types:
//
//	response.created, response.in_progress, response.completed, response.failed
//	response.output_item.added, response.output_item.done
//	response.content_part.added, response.content_part.done
//	response.output_text.delta, response.output_text.annotation.added, response.output_text.done
//	response.refusal.delta, response.refusal.done
//	response.function_call_arguments.delta, response.function_call_arguments.done
//	error
type ResponseStreamEvent struct {
	// Type discriminator for the event kind.
	Type string `json:"type"`
	// Response is included in response.created, response.completed, response.failed events.
	Response *Response `json:"response,omitempty"`
	// Item is included in response.output_item.* events.
	Item *ResponseOutputItem `json:"item,omitempty"`
	// ContentPart is included in response.content_part.* events.
	ContentPart *ResponseContentPart `json:"content_part,omitempty"`
	// Delta is a text chunk for response.output_text.delta events.
	Delta string `json:"delta,omitempty"`
	// Text is the full text for response.output_text.done events.
	Text string `json:"text,omitempty"`
	// Message is the error message for error events.
	Message string `json:"message,omitempty"`
	// Code is the error code for error events.
	Code string `json:"code,omitempty"`
}
