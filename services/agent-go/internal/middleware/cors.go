// Package middleware provides HTTP middleware for the JARVIS agent server.
package middleware

import "net/http"

// CORSMiddleware adds permissive CORS headers for local development.
// In production, the API gateway (Fastify) handles CORS; this is for
// direct frontend calls during dev (e.g. GET /suggestions).
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Tenant-ID")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
