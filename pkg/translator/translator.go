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
	StatusCode       int
	Body             []byte
	Headers          map[string]string
	ReasoningContent string // extracted from upstream response for session preservation
}

type Translator interface {
	TranslateRequest(ctx context.Context, req *Request, s *session.Session) (*UpstreamRequest, error)
	TranslateStream(ctx context.Context, upstream io.Reader, req *Request, s *session.Session) <-chan SSEEvent
	TranslateResponse(ctx context.Context, upstream *http.Response, req *Request, s *session.Session) (*Response, error)
	UpdateSession(s *session.Session, req *Request, resp *Response)
}
