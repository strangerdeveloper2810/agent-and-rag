// Package http là lớp transport: HTTP handlers (health, chat SSE, resume).
package http

import "net/http"

// Healthz — liveness (process còn sống).
func Healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, `{"status":"ok"}`)
}

func writeJSON(w http.ResponseWriter, code int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write([]byte(body))
}
