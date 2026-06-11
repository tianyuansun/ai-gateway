package translator

import (
	"context"
	"io"

	"github.com/tianyuansun/ai-gateway/session"
)

type APIFormat string

const (
	FormatResponses APIFormat = "responses"
	FormatChat      APIFormat = "chat"
	FormatAnthropic APIFormat = "anthropic"
)

type Request struct {
	Model      string
	APIFormat  APIFormat
	Body       []byte
	Headers    map[string]string
	QueryParams map[string]string
}

type UpstreamRequest struct {
	Method  string
	URL     string
	Body    []byte
	Headers map[string]string
}

type SSEEvent struct {
	Event string
	Data  []byte
}

type Response struct {
	StatusCode int
	Body       []byte
	Headers    map[string]string
}

type Translator interface {
	TranslateRequest(ctx context.Context, req *Request, s *session.Session) (*UpstreamRequest, error)
	TranslateStream(ctx context.Context, upstream io.Reader, req *Request, s *session.Session) <-chan SSEEvent
	TranslateResponse(ctx context.Context, upstreamBody []byte, req *Request, s *session.Session) (*Response, error)
	UpdateSession(s *session.Session, req *Request, resp *Response)
}

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
		data, _ := io.ReadAll(upstream)
		ch <- SSEEvent{Data: data}
	}()
	return ch
}

func (p *PassthroughTranslator) TranslateResponse(_ context.Context, upstreamBody []byte, _ *Request, _ *session.Session) (*Response, error) {
	return &Response{StatusCode: 200, Body: upstreamBody}, nil
}

func (p *PassthroughTranslator) UpdateSession(_ *session.Session, _ *Request, _ *Response) {}
