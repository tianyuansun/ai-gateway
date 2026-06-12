package ingress

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/tianyuansun/ai-gateway/pkg/config"
	"github.com/tianyuansun/ai-gateway/pkg/provider"
	"github.com/tianyuansun/ai-gateway/pkg/router"
	"github.com/tianyuansun/ai-gateway/pkg/session"
	"github.com/tianyuansun/ai-gateway/pkg/translator"
)

type Gateway struct {
	cfg      *config.Config
	resolver *router.ModelResolver
	selector *router.ProviderSelector
	client   *provider.Client
	sessions session.Store
	health   *provider.HealthChecker

	translators map[translatorKey]translator.Translator
}

type translatorKey struct {
	exposed  translator.APIFormat
	provider string // "chat", "anthropic", "responses"
}

func NewGateway(cfg *config.Config) *Gateway {
	sessStore := session.NewMemoryStore(10000, 3600*time.Second)
	checker := provider.NewHealthChecker(30)
	sel := router.NewProviderSelector(cfg, sessStore, checker)

	gw := &Gateway{
		cfg:         cfg,
		resolver:    router.NewModelResolver(cfg),
		selector:    sel,
		client:      provider.NewClient(120 * time.Second),
		sessions:    sessStore,
		health:      checker,
		translators: make(map[translatorKey]translator.Translator),
	}

	gw.translators[translatorKey{"responses", "chat"}] = &translator.ResToChat{}
	gw.translators[translatorKey{"responses", "anthropic"}] = &translator.ResToAnth{}
	gw.translators[translatorKey{"anthropic", "chat"}] = &translator.AnthToChat{}
	gw.translators[translatorKey{"chat", "anthropic"}] = &translator.ChatToAnth{}

	return gw
}

func (gw *Gateway) HealthChecker() *provider.HealthChecker {
	return gw.health
}

func (gw *Gateway) Start() error {
	// Start health checker
	providers := make(map[string]provider.ProviderEndpoint)
	for id, p := range gw.cfg.Providers {
		baseURL := p.Endpoints.Chat
		if baseURL == "" {
			baseURL = p.Endpoints.Anthropic
		}
		providers[id] = provider.ProviderEndpoint{BaseURL: baseURL}
	}
	gw.health.Start(context.Background(), providers)

	return nil
}

// ServeResponses handles POST /v1/responses (Codex CLI)
func (gw *Gateway) ServeResponses(w http.ResponseWriter, r *http.Request) {
	gw.handleProxy(w, r, translator.FormatResponses)
}

// ServeChat handles POST /v1/chat/completions
func (gw *Gateway) ServeChat(w http.ResponseWriter, r *http.Request) {
	gw.handleProxy(w, r, translator.FormatChat)
}

// ServeMessages handles POST /v1/messages (Claude Code)
func (gw *Gateway) ServeMessages(w http.ResponseWriter, r *http.Request) {
	gw.handleProxy(w, r, translator.FormatAnthropic)
}

func (gw *Gateway) handleProxy(w http.ResponseWriter, r *http.Request, apiFormat translator.APIFormat) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	modelName := extractModel(body, apiFormat)
	if modelName == "" {
		http.Error(w, "model not found in request", http.StatusBadRequest)
		return
	}

	model, canonicalName, ok := gw.resolver.Resolve(modelName)
	if !ok {
		http.Error(w, fmt.Sprintf("model %q not configured", modelName), http.StatusNotFound)
		return
	}

	sessionID := extractSessionID(r, body, apiFormat)
	if sessionID == "" {
		sessionID = generateSessionID()
	}

	prov, provID, err := gw.selector.Select(model, sessionID)
	if err != nil {
		http.Error(w, "no provider available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	tr, endpoint := gw.resolveTranslator(apiFormat, prov)
	if tr == nil {
		http.Error(w, "no translator for this path", http.StatusInternalServerError)
		return
	}

	var sess *session.Session
	if sessionID != "" {
		sess, _ = gw.sessions.Get(sessionID)
	}
	if sess == nil {
		sess = &session.Session{ID: sessionID, TTL: 3600 * time.Second}
	}

	tReq := &translator.Request{
		Model:     canonicalName,
		APIFormat: apiFormat,
		Body:      body,
		Headers:   flattenHeaders(r.Header),
	}

	upReq, err := tr.TranslateRequest(r.Context(), tReq, sess)
	if err != nil {
		http.Error(w, "translate request: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if upReq.URL == "" {
		upReq.URL = endpoint
	}
	baseURL := prov.Endpoints.Chat
	if strings.Contains(endpoint, "/messages") {
		baseURL = prov.Endpoints.Anthropic
	}
	if baseURL == "" {
		baseURL = prov.Endpoints.Chat
	}

	apiKey := gw.cfg.ProviderAPIKey(prov)

	resp, err := gw.client.Call(r.Context(), baseURL, upReq.URL, apiKey, upReq.Body, upReq.Headers)
	if err != nil {
		log.Printf("[gateway] upstream error: %v", err)
		http.Error(w, "upstream error: "+err.Error(), http.StatusBadGateway)
		return
	}

	gwResp, err := tr.TranslateResponse(r.Context(), resp, tReq, sess)
	if err != nil {
		http.Error(w, "translate response: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if sess != nil {
		sess.ProviderID = provID
		sess.ModelName = canonicalName
		tr.UpdateSession(sess, tReq, gwResp)
		gw.sessions.Set(sess.ID, sess)
	}

	w.Header().Set("X-Session-Id", sessionID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(gwResp.StatusCode)
	w.Write(gwResp.Body)
}

func (gw *Gateway) resolveTranslator(apiFormat translator.APIFormat, prov *config.Provider) (translator.Translator, string) {
	switch apiFormat {
	case translator.FormatResponses:
		if prov.Endpoints.Anthropic != "" {
			return gw.translators[translatorKey{"responses", "anthropic"}], "/messages"
		}
		return gw.translators[translatorKey{"responses", "chat"}], "/chat/completions"

	case translator.FormatAnthropic:
		if prov.Endpoints.Anthropic != "" {
			return &translator.PassthroughTranslator{}, "/messages"
		}
		return gw.translators[translatorKey{"anthropic", "chat"}], "/chat/completions"

	case translator.FormatChat:
		if prov.Endpoints.Chat != "" {
			return &translator.PassthroughTranslator{}, "/chat/completions"
		}
		return gw.translators[translatorKey{"chat", "anthropic"}], "/messages"
	}
	return nil, ""
}

func extractModel(body []byte, apiFormat translator.APIFormat) string {
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return ""
	}
	if m, ok := data["model"].(string); ok {
		return m
	}
	return ""
}

func extractSessionID(r *http.Request, body []byte, apiFormat translator.APIFormat) string {
	// 1. Check X-Session-Id header
	if sid := r.Header.Get("X-Session-Id"); sid != "" {
		return sid
	}

	// 2. For Responses API, extract previous_response_id
	if apiFormat == translator.FormatResponses {
		var data map[string]interface{}
		if json.Unmarshal(body, &data) == nil {
			if prid, ok := data["previous_response_id"].(string); ok && prid != "" {
				return prid
			}
		}
	}
	return ""
}

func generateSessionID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return "gw-" + hex.EncodeToString(b)
}

func flattenHeaders(h http.Header) map[string]string {
	result := make(map[string]string)
	for k, v := range h {
		if k != "Host" && k != "Authorization" {
			result[k] = v[0]
		}
	}
	return result
}
