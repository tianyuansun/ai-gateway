package logging

import "time"

// FlushConfig holds the post-request conditions that determine whether
// buffered logs should be flushed or discarded.
type FlushConfig struct {
	Threshold time.Duration // latency threshold (0 = disabled)
	XDebug    bool          // X-Debug: true header was present
}

// ShouldFlush returns true if any trigger condition is met.
// latency is the total request duration; status is the HTTP status; err is the upstream error.
func (c FlushConfig) ShouldFlush(latency time.Duration, status int, err error) bool {
	if c.XDebug {
		return true
	}
	if status >= 500 {
		return true
	}
	if err != nil {
		return true
	}
	if c.Threshold > 0 && latency > c.Threshold {
		return true
	}
	return false
}
