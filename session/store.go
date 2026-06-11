package session

import "time"

type Message struct {
	Role    string     `json:"role"`
	Content string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string  `json:"tool_call_id,omitempty"`
	Name      string   `json:"name,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type Reasoning struct {
	Content string `json:"content"`
}

type Session struct {
	ID               string      `json:"id"`
	ModelName        string      `json:"model_name"`
	ProviderID       string      `json:"provider_id"`
	Messages         []Message   `json:"messages"`
	ReasoningRecords []Reasoning `json:"reasoning_records"`
	CreatedAt        time.Time   `json:"created_at"`
	LastAccess       time.Time   `json:"last_access"`
	TTL              time.Duration `json:"ttl"`
}

type Store interface {
	Get(sessionID string) (*Session, error)
	Set(sessionID string, s *Session) error
	Delete(sessionID string) error
	Prune()
}
