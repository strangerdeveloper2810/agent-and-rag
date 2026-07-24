// Package http là lớp transport: HTTP handlers (health, chat SSE, resume).
package http

import (
	"encoding/json"
	"net/http"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// MongoPinger is an interface for MongoDB health checks.
type MongoPinger interface {
	Ping() error
}

// Healthz — liveness (process còn sống).
func Healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, `{"status":"ok"}`)
}

// NewReadyzHandler returns a handler that checks provider + MongoDB status.
func NewReadyzHandler(prov provider.Provider, mongo MongoPinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		checks := map[string]string{
			"provider": prov.Name(),
		}
		healthy := true

		if mongo != nil {
			if err := mongo.Ping(); err != nil {
				checks["mongodb"] = "error: " + err.Error()
				healthy = false
			} else {
				checks["mongodb"] = "ok"
			}
		} else {
			checks["mongodb"] = "not configured"
		}

		status := map[string]any{
			"status": "ok",
			"checks": checks,
		}
		if !healthy {
			status["status"] = "degraded"
		}

		w.Header().Set("Content-Type", "application/json")
		if healthy {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(status)
	}
}

func writeJSON(w http.ResponseWriter, code int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write([]byte(body))
}
