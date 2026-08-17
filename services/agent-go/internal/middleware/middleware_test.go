package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// okHandler ghi 200 + "ok" và ghi lại context nó nhận được.
func okHandler(seen *context.Context) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if seen != nil {
			*seen = r.Context()
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

func TestCORSMiddleware_SetsHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/suggestions", nil)

	CORSMiddleware(okHandler(nil)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("body = %q, want ok (handler phải được gọi)", rec.Body.String())
	}

	want := map[string]string{
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Methods": "GET, POST, OPTIONS",
		"Access-Control-Allow-Headers": "Content-Type, Authorization, X-Tenant-ID",
	}
	for k, v := range want {
		if got := rec.Header().Get(k); got != v {
			t.Errorf("header %s = %q, want %q", k, got, v)
		}
	}
}

// Preflight OPTIONS phải trả 204 và KHÔNG gọi handler phía sau.
func TestCORSMiddleware_PreflightShortCircuits(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/chat", nil)

	called := false
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })

	CORSMiddleware(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
	if called {
		t.Error("handler phía sau bị gọi trên preflight OPTIONS")
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("preflight thiếu header CORS")
	}
}

func TestTenantMiddleware_UsesHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/chat", nil)
	req.Header.Set("X-Tenant-ID", "acme")

	var seen context.Context
	TenantMiddleware(okHandler(&seen)).ServeHTTP(rec, req)

	if got := GetTenantID(seen); got != "acme" {
		t.Errorf("GetTenantID = %q, want acme", got)
	}
}

func TestTenantMiddleware_DefaultsWhenHeaderMissing(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/chat", nil)

	var seen context.Context
	TenantMiddleware(okHandler(&seen)).ServeHTTP(rec, req)

	if got := GetTenantID(seen); got != "default" {
		t.Errorf("GetTenantID = %q, want default", got)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestGetTenantID_Fallbacks(t *testing.T) {
	if got := GetTenantID(context.Background()); got != "default" {
		t.Errorf("context rỗng: GetTenantID = %q, want default", got)
	}

	// Giá trị sai kiểu cũng phải rơi về "default", không panic.
	ctx := context.WithValue(context.Background(), TenantIDKey, 42)
	if got := GetTenantID(ctx); got != "default" {
		t.Errorf("giá trị sai kiểu: GetTenantID = %q, want default", got)
	}
}

// Key phải là kiểu riêng (contextKey), không phải string thuần — tránh đụng key.
func TestTenantIDKey_IsUnexportedType(t *testing.T) {
	ctx := context.WithValue(context.Background(), "tenant_id", "raw-string-key") //nolint:staticcheck // cố ý dùng string key
	if got := GetTenantID(ctx); got != "default" {
		t.Errorf("string key không được đụng contextKey: GetTenantID = %q, want default", got)
	}
}

func TestMiddlewareChain(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/chat", nil)
	req.Header.Set("X-Tenant-ID", "t1")

	var seen context.Context
	handler := CORSMiddleware(TenantMiddleware(okHandler(&seen)))
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("chain mất header CORS")
	}
	if got := GetTenantID(seen); got != "t1" {
		t.Errorf("chain: GetTenantID = %q, want t1", got)
	}
}
