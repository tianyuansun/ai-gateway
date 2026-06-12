package logging

import (
	"encoding/json"
	"net/http"
	"log/slog"
	"strings"
)

// levelNames maps slog levels to their string representations.
var levelNames = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

// levelName reverses levelNames.
func levelName(l slog.Level) string {
	switch l {
	case slog.LevelDebug:
		return "DEBUG"
	case slog.LevelInfo:
		return "INFO"
	case slog.LevelWarn:
		return "WARN"
	case slog.LevelError:
		return "ERROR"
	default:
		return "INFO"
	}
}

// AdminHandler returns an http.Handler that manages the global log level.
// Mount at "/admin/log-level" or "/admin/" with prefix stripping.
func AdminHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"level": levelName(GlobalLevel()),
			})

		case http.MethodPut:
			var body struct {
				Level string `json:"level"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, `invalid JSON`, http.StatusBadRequest)
				return
			}
			lvl, ok := levelNames[strings.ToLower(body.Level)]
			if !ok {
				http.Error(w, `invalid level; must be debug, info, warn, or error`, http.StatusBadRequest)
				return
			}
			SetGlobalLevel(lvl)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"level": levelName(GlobalLevel()),
			})

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}
