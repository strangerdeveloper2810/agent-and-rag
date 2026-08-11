// Package middleware provides HTTP middleware for the JARVIS agent server.
package middleware

import (
	"context"
	"net/http"
)

// contextKey is an unexported type used for context keys to avoid collisions.
type contextKey string

// TenantIDKey is the context key for the tenant ID value.
const TenantIDKey contextKey = "tenant_id"

// TenantMiddleware extracts the X-Tenant-ID header from incoming requests,
// falls back to "default" if not present, and stores it in the request context.
func TenantMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Header.Get("X-Tenant-ID")
		if tenantID == "" {
			tenantID = "default"
		}
		ctx := context.WithValue(r.Context(), TenantIDKey, tenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetTenantID retrieves the tenant ID from the context.
// Returns "default" if no tenant ID was set.
func GetTenantID(ctx context.Context) string {
	if id, ok := ctx.Value(TenantIDKey).(string); ok {
		return id
	}
	return "default"
}
