package provider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClient_Call_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	client := NewClient(10 * time.Second)
	resp, err := client.Call(context.Background(), srv.URL, "/v1/chat", "sk-test", []byte(`{"prompt":"hello"}`), nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	if string(body) != `{"ok":true}` {
		t.Errorf("expected body '{\"ok\":true}', got '%s'", string(body))
	}
}

func TestClient_Call_NetworkError(t *testing.T) {
	client := NewClient(1 * time.Second)
	_, err := client.Call(context.Background(), "http://127.0.0.1:19999", "/v1/chat", "sk-test", []byte(`{}`), nil)

	if err == nil {
		t.Fatal("expected error for connection refused, got nil")
	}
	if !strings.Contains(err.Error(), "upstream call") {
		t.Errorf("expected error to wrap 'upstream call', got: %v", err)
	}
}
