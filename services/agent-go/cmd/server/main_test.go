package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/agent"
	"github.com/ai-agent-tut/agent-go/internal/config"
	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// stubRunner trả về text cố định, không gọi LLM.
type stubRunner struct{ text string }

func (s *stubRunner) Run(_ context.Context, _ agent.RunInput, emit agent.EmitFunc) (provider.Usage, error) {
	if s.text != "" {
		emit(agent.TextEvent(s.text))
	}
	emit(agent.DoneEvent(provider.Usage{}, 0, false))
	return provider.Usage{}, nil
}

func toolNames(t *testing.T, defs []provider.ToolDef) map[string]bool {
	t.Helper()
	out := make(map[string]bool, len(defs))
	for _, d := range defs {
		out[d.Name] = true
	}
	return out
}

func TestBuildRegistries_ScopedPerSpecialty(t *testing.T) {
	cfg := config.Config{AllowedPaths: []string{t.TempDir()}}
	code, research, general := buildRegistries(cfg)

	codeTools := toolNames(t, code.ToolDefs())
	for _, want := range []string{"file.read", "file.write", "shell.exec", "git", "version"} {
		if !codeTools[want] {
			t.Errorf("code registry thiếu %q (có: %v)", want, codeTools)
		}
	}
	// Agent code KHÔNG được cầm tool tìm kiếm web.
	if codeTools["web.search"] {
		t.Error("code registry không nên có web.search")
	}

	researchTools := toolNames(t, research.ToolDefs())
	for _, want := range []string{"web.search", "web.fetch", "notes.search", "notes.create"} {
		if !researchTools[want] {
			t.Errorf("research registry thiếu %q (có: %v)", want, researchTools)
		}
	}
	if researchTools["shell.exec"] {
		t.Error("research registry không nên có shell.exec")
	}

	generalTools := toolNames(t, general.ToolDefs())
	for _, want := range []string{"echo", "web.search", "calculator", "datetime", "translate"} {
		if !generalTools[want] {
			t.Errorf("general registry thiếu %q (có: %v)", want, generalTools)
		}
	}

	// Tool memory phải có ở cả 3 agent.
	for name, names := range map[string]map[string]bool{
		"code": codeTools, "research": researchTools, "general": generalTools,
	} {
		if !names["memory.save"] || !names["memory.recall"] {
			t.Errorf("%s registry thiếu tool memory: %v", name, names)
		}
	}
}

func TestNewHTTPHandler_Routes(t *testing.T) {
	h := newHTTPHandler(provider.NewFake(), &stubRunner{text: `["a"]`}, nil, nil)

	cases := []struct {
		name, method, path string
		body               string
		wantStatus         int
	}{
		{"healthz", http.MethodGet, "/healthz", "", http.StatusOK},
		{"readyz", http.MethodGet, "/readyz", "", http.StatusOK},
		{"chat", http.MethodPost, "/chat", `{"userMessage":"hi"}`, http.StatusOK},
		{"suggestions", http.MethodGet, "/suggestions", "", http.StatusOK},
		{"route lạ", http.MethodGet, "/không-có", "", http.StatusNotFound},
		{"chat sai method", http.MethodGet, "/chat", "", http.StatusMethodNotAllowed},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d (body: %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestNewHTTPHandler_MiddlewareChain(t *testing.T) {
	h := newHTTPHandler(provider.NewFake(), &stubRunner{}, nil, nil)

	// CORS: preflight trả 204 kèm header.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodOptions, "/chat", nil))
	if rec.Code != http.StatusNoContent {
		t.Errorf("preflight status = %d, want 204", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("thiếu header CORS")
	}

	// Request thường vẫn đi qua được cả chuỗi middleware.
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Tenant-ID", "acme")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// Chưa nối Mongo → readyz báo "not configured" và vẫn 200 (không panic vì
// interface nil đúng nghĩa).
func TestNewHTTPHandler_ReadyzWithoutMongo(t *testing.T) {
	h := newHTTPHandler(provider.NewFake(), &stubRunner{}, mongoPinger(nil), nil)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Checks map[string]string `json:"checks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse body: %v", err)
	}
	if body.Checks["mongodb"] != "not configured" {
		t.Errorf("checks.mongodb = %q, want not configured", body.Checks["mongodb"])
	}
}
