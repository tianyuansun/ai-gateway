package provider

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"
)

type HealthChecker struct {
	mu       sync.RWMutex
	status   map[string]bool
	client   *http.Client
	interval time.Duration
}

type ProviderEndpoint struct {
	BaseURL string
}

func NewHealthChecker(interval time.Duration) *HealthChecker {
	return &HealthChecker{
		status:   make(map[string]bool),
		client:   &http.Client{Timeout: 10 * time.Second},
		interval: interval,
	}
}

func (h *HealthChecker) Start(ctx context.Context, providers map[string]ProviderEndpoint) {
	go func() {
		ticker := time.NewTicker(h.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				h.probeAll(providers)
			}
		}
	}()
}

func (h *HealthChecker) probeAll(providers map[string]ProviderEndpoint) {
	var wg sync.WaitGroup
	for id, p := range providers {
		wg.Add(1)
		go func(providerID, baseURL string) {
			defer wg.Done()
			h.probeOne(providerID, baseURL)
		}(id, p.BaseURL)
	}
	wg.Wait()
}

func (h *HealthChecker) probeOne(providerID, baseURL string) {
	healthy := false
	defer func() {
		h.mu.Lock()
		h.status[providerID] = healthy
		h.mu.Unlock()
	}()

	resp, err := h.client.Get(baseURL + "/models")
	if err != nil {
		log.Printf("[health] provider %s: %v", providerID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 || resp.StatusCode == 401 || resp.StatusCode == 403 {
		// 401/403 means endpoint is reachable, just auth issue
		healthy = true
	}
}

func (h *HealthChecker) IsHealthy(providerID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if status, ok := h.status[providerID]; ok {
		return status
	}
	return true // default healthy
}

func (h *HealthChecker) SetHealth(providerID string, healthy bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.status[providerID] = healthy
}

func (h *HealthChecker) Status() map[string]bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make(map[string]bool, len(h.status))
	for k, v := range h.status {
		result[k] = v
	}
	return result
}
