package translator

import (
	"context"
	"io"

	"github.com/tianyuansun/ai-gateway/pkg/session"
	"github.com/tianyuansun/ai-gateway/pkg/shared"
)

// PassthroughTranslator passes requests and responses through unchanged.
// Used when the exposed API format matches the upstream provider's native format.
type PassthroughTranslator struct{}

func (p *PassthroughTranslator) TranslateRequest(_ context.Context, req *Request, _ *session.Session) (*UpstreamRequest, error) {
	return &UpstreamRequest{
		Method:  "POST",
		URL:     "",
		Body:    req.Body,
		Headers: req.Headers,
	}, nil
}

func (p *PassthroughTranslator) TranslateStream(_ context.Context, upstream io.Reader, _ *Request, _ *session.Session) <-chan SSEEvent {
	ch := make(chan SSEEvent)
	go func() {
		defer close(ch)
		for ev := range shared.ParseSSE(upstream) {
			ch <- SSEEvent{
				Event: ev.Event,
				Data:  []byte(ev.Data),
			}
		}
	}()
	return ch
}
