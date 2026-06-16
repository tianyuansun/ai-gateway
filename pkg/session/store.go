package session

import "time"

// Session holds minimal cross-turn state for provider affinity.
// Message history and reasoning are NOT stored — every coding agent
// sends the full conversation in each request body.
// See docs/adr/0006-session-store-stateless.md.
type Session struct {
	ID         string        `json:"id"`
	ModelName  string        `json:"model_name"`
	ProviderID string        `json:"provider_id"`
	CreatedAt  time.Time     `json:"created_at"`
	LastAccess time.Time     `json:"last_access"`
	TTL        time.Duration `json:"ttl"`
}

type Store interface {
	Get(sessionID string) (*Session, error)
	Set(sessionID string, s *Session) error
	Delete(sessionID string) error
	Prune()
}
