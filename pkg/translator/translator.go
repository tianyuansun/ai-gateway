package translator

import (
	"context"
	"io"
	"net/http"

	"github.com/tianyuansun/ai-gateway/pkg/session"
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
	TranslateResponse(ctx context.Context, upstream *http.Response, req *Request, s *session.Session) (*Response, error)
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

func (p *PassthroughTranslator) TranslateResponse(_ context.Context, upstream *http.Response, _ *Request, _ *session.Session) (*Response, error) {
	body, err := io.ReadAll(upstream.Body)
	if err != nil {
		return nil, err
	}
	upstream.Body.Close()
	return &Response{StatusCode: upstream.StatusCode, Body: body}, nil
}

func (p *PassthroughTranslator) UpdateSession(_ *session.Session, _ *Request, _ *Response) {}
