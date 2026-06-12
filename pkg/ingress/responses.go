package ingress

import (
	"net/http"

	"github.com/tianyuansun/ai-gateway/pkg/translator"
)

// ServeResponses handles POST /v1/responses (Codex CLI).
func (gw *Gateway) ServeResponses(w http.ResponseWriter, r *http.Request) {
	gw.handleProxy(w, r, translator.FormatResponses)
}
