package provider

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthChecker_ProbeSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h := NewHealthChecker(10 * time.Second)
	h.probeOne("test-provider", srv.URL)

	if !h.IsHealthy("test-provider") {
		t.Error("expected IsHealthy true after successful probe")
	}
}

func TestHealthChecker_ProbeFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	h := NewHealthChecker(10 * time.Second)
	h.probeOne("test-provider", srv.URL)

	if h.IsHealthy("test-provider") {
		t.Error("expected IsHealthy false after 500 probe")
	}
}

func TestHealthChecker_SetHealth(t *testing.T) {
	h := NewHealthChecker(10 * time.Second)

	// Default healthy for unknown provider
	if !h.IsHealthy("provider-a") {
		t.Error("expected IsHealthy true for unknown provider (default)")
	}

	// Set to false
	h.SetHealth("provider-a", false)
	if h.IsHealthy("provider-a") {
		t.Error("expected IsHealthy false after SetHealth(false)")
	}

	// Set back to true
	h.SetHealth("provider-a", true)
	if !h.IsHealthy("provider-a") {
		t.Error("expected IsHealthy true after SetHealth(true)")
	}
}
