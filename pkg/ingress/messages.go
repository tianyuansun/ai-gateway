package ingress

import (
	"net/http"

	"github.com/tianyuansun/ai-gateway/pkg/translator"
)

// ServeMessages handles POST /v1/messages (Claude Code).
func (gw *Gateway) ServeMessages(w http.ResponseWriter, r *http.Request) {
	gw.handleProxy(w, r, translator.FormatAnthropic)
}
