package ingress

import (
	"net/http"

	"github.com/tianyuansun/ai-gateway/pkg/translator"
)

// ServeChat handles POST /v1/chat/completions.
func (gw *Gateway) ServeChat(w http.ResponseWriter, r *http.Request) {
	gw.handleProxy(w, r, translator.FormatChat)
}
