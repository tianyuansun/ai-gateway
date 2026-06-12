package ingress

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/tianyuansun/ai-gateway/pkg/config"
	"github.com/tianyuansun/ai-gateway/pkg/logging"
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
	// Set global log level from config.
	logging.SetGlobalLevel(logging.ParseLevel(gw.cfg.Server.LogLevel))

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

func (gw *Gateway) handleProxy(w http.ResponseWriter, r *http.Request, apiFormat translator.APIFormat) {
	// Per-request buffered logging.
	startTime := time.Now()
	requestID := generateRequestID()
	xDebug := r.Header.Get("X-Debug") == "true"
	level := logging.GlobalLevel()
	if xDebug {
		level = slog.LevelDebug
	}
	ctx, logBuf := logging.WithLogger(r.Context(), requestID, level)

	var upstreamStatus int
	var upstreamErr error
	defer func() {
		cfg := logging.FlushConfig{
			Threshold: time.Duration(gw.cfg.Server.LogLatencyThresholdMs) * time.Millisecond,
			XDebug:    xDebug,
		}
		latency := time.Since(startTime)
		if cfg.ShouldFlush(latency, upstreamStatus, upstreamErr) {
			logBuf.Flush(os.Stderr)
		} else {
			logBuf.Discard()
		}
	}()

	logger := logging.LoggerFrom(ctx)

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

	streamReq := isStreamRequest(body)
	logger.Debug("request",
		"method", r.Method,
		"path", r.URL.Path,
		"model", modelName,
		"stream", streamReq,
		"body_size", len(body),
	)

	model, canonicalName, ok := gw.resolver.Resolve(modelName)
	if !ok {
		http.Error(w, fmt.Sprintf("model %q not configured", modelName), http.StatusNotFound)
		return
	}
	if canonicalName != modelName {
		logger.Debug("model_resolved", "alias", modelName, "canonical", canonicalName)
	}

	sessionID := extractSessionID(r, body, apiFormat)
	if sessionID == "" {
		sessionID = generateSessionID()
	}
	var sessionHit bool
	var sess *session.Session
	if sessionID != "" {
		sess, _ = gw.sessions.Get(sessionID)
	}
	if sess != nil {
		sessionHit = true
	} else {
		sess = &session.Session{ID: sessionID, TTL: 3600 * time.Second}
	}
	logger.Debug("session_lookup", "session_id", sessionID, "hit", sessionHit)

	prov, provID, err := gw.selector.Select(model, sessionID)
	if err != nil {
		http.Error(w, "no provider available: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	logger.Debug("provider_selected",
		"strategy", safeStrategy(model.Routing),
		"provider", provID,
	)

	tr, endpoint := gw.resolveTranslator(apiFormat, prov)
	if tr == nil {
		http.Error(w, "no translator for this path", http.StatusInternalServerError)
		return
	}
	logger.Debug("translator_selected",
		"exposed_format", apiFormat,
		"endpoint", endpoint,
	)

	tReq := &translator.Request{
		Model:     canonicalName,
		APIFormat: apiFormat,
		Body:      body,
		Headers:   flattenHeaders(r.Header),
	}

	translateStart := time.Now()
	upReq, err := tr.TranslateRequest(ctx, tReq, sess)
	translateLatency := time.Since(translateStart)
	if err != nil {
		http.Error(w, "translate request: "+err.Error(), http.StatusInternalServerError)
		return
	}
	logger.Debug("translate_request",
		"latency_ms", translateLatency.Milliseconds(),
		"body_size_in", len(body),
		"body_size_out", len(upReq.Body),
	)

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

	upstreamStart := time.Now()
	resp, err := gw.client.Call(ctx, baseURL, upReq.URL, apiKey, upReq.Body, upReq.Headers)
	upstreamLatency := time.Since(upstreamStart)
	if err != nil {
		upstreamErr = err
		logger.Error("upstream_error", "latency_ms", upstreamLatency.Milliseconds(), "error", err.Error())
		http.Error(w, "upstream error: "+err.Error(), http.StatusBadGateway)
		return
	}
	upstreamStatus = resp.StatusCode
	logger.Info("upstream_call",
		"base_url", baseURL,
		"path", upReq.URL,
		"status", upstreamStatus,
		"latency_ms", upstreamLatency.Milliseconds(),
	)

	if isStreamRequest(body) {
		gw.handleStream(w, r, resp, ctx, tr, tReq, sess, sessionID, provID, canonicalName)
	} else {
		gw.handleNonStream(w, r, resp, ctx, tr, tReq, sess, sessionID, provID, canonicalName)
	}
	logger.Info("response",
		"status", upstreamStatus,
		"latency_ms", time.Since(startTime).Milliseconds(),
	)
}

func (gw *Gateway) handleStream(w http.ResponseWriter, r *http.Request, upstream *http.Response, ctx context.Context, tr translator.Translator, tReq *translator.Request, sess *session.Session, sessionID, provID, canonicalName string) {
	defer upstream.Body.Close()

	logger := logging.LoggerFrom(ctx)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Session-Id", sessionID)
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}

	eventCount := 0
	for sseEv := range tr.TranslateStream(r.Context(), upstream.Body, tReq, sess) {
		eventCount++
		if sseEv.Event != "" {
			fmt.Fprintf(w, "event: %s\n", sseEv.Event)
		}
		fmt.Fprintf(w, "data: %s\n\n", sseEv.Data)
		flusher.Flush()
	}

	if sess != nil {
		sess.ProviderID = provID
		sess.ModelName = canonicalName
		gw.sessions.Set(sess.ID, sess)
	}

	if logger != nil {
		logger.Debug("translate_stream",
			"event_count", eventCount,
		)
		logger.Debug("session_update",
			"provider_bound", provID,
			"model", canonicalName,
		)
	}
}

func (gw *Gateway) handleNonStream(w http.ResponseWriter, r *http.Request, upstream *http.Response, ctx context.Context, tr translator.Translator, tReq *translator.Request, sess *session.Session, sessionID, provID, canonicalName string) {
	defer upstream.Body.Close()

	logger := logging.LoggerFrom(ctx)

	// Drain the streaming channel and accumulate into a full Response.
	var responseID string
	var outputText string
	eventCount := 0
	for sseEv := range tr.TranslateStream(r.Context(), upstream.Body, tReq, sess) {
		eventCount++
		switch sseEv.Event {
		case "response.created":
			var data struct {
				Response struct {
					ID string `json:"id"`
				} `json:"response"`
			}
			if json.Unmarshal(sseEv.Data, &data) == nil && data.Response.ID != "" {
				responseID = data.Response.ID
			}
		case "response.output_text.delta":
			var data struct {
				Delta string `json:"delta"`
			}
			if json.Unmarshal(sseEv.Data, &data) == nil {
				outputText += data.Delta
			}
		default:
			// Passthrough / raw data events (no event type, just Chat chunks).
			var chunk struct {
				ID      string `json:"id"`
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				} `json:"choices"`
			}
			if json.Unmarshal(sseEv.Data, &chunk) == nil {
				if chunk.ID != "" && responseID == "" {
					responseID = chunk.ID
				}
				for _, c := range chunk.Choices {
					if c.Delta.Content != "" {
						outputText += c.Delta.Content
					}
					if c.Message.Content != "" {
						outputText += c.Message.Content
					}
				}
			}
		}
	}

	respBody, _ := json.Marshal(map[string]any{
		"id":     responseID,
		"object": "response",
		"output": []map[string]any{{
			"type": "message",
			"role": "assistant",
			"content": []map[string]any{{
				"type": "output_text",
				"text": outputText,
			}},
		}},
	})

	if sess != nil {
		sess.ProviderID = provID
		sess.ModelName = canonicalName
		gw.sessions.Set(sess.ID, sess)
	}

	if logger != nil {
		logger.Debug("translate_response",
			"event_count", eventCount,
			"body_size", len(respBody),
		)
		logger.Debug("session_update",
			"provider_bound", provID,
			"model", canonicalName,
		)
	}

	w.Header().Set("X-Session-Id", sessionID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respBody)
}

func isStreamRequest(body []byte) bool {
	var data struct {
		Stream bool `json:"stream"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return false
	}
	return data.Stream
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

func generateRequestID() string {
	b := make([]byte, 8)
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

func safeStrategy(r *config.RoutingConfig) string {
	if r == nil {
		return "priority"
	}
	return r.Strategy
}
