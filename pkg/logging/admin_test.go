package logging

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdminHandlerGetLevel(t *testing.T) {
	SetGlobalLevel(slog.LevelInfo)
	handler := AdminHandler()

	req := httptest.NewRequest(http.MethodGet, "/admin/log-level", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["level"] != "INFO" {
		t.Errorf("expected level INFO, got %s", body["level"])
	}
}

func TestAdminHandlerPutLevelValid(t *testing.T) {
	SetGlobalLevel(slog.LevelInfo)

	req := httptest.NewRequest(http.MethodPut, "/admin/log-level",
		strings.NewReader(`{"level":"debug"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	AdminHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if GlobalLevel() != slog.LevelDebug {
		t.Errorf("expected debug level, got %v", GlobalLevel())
	}
}

func TestAdminHandlerPutLevelInvalid(t *testing.T) {
	SetGlobalLevel(slog.LevelInfo)

	req := httptest.NewRequest(http.MethodPut, "/admin/log-level",
		strings.NewReader(`{"level":"nogood"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	AdminHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid level, got %d", rec.Code)
	}
}
