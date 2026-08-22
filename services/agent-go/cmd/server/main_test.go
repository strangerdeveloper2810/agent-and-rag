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
	"github.com/ai-agent-tut/agent-go/internal/memory"
	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/tools"
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
	code, research, general := buildRegistries(cfg, memory.NewStore())

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

	// Tool memory phải có ở cả 3 agent — kể cả memory.list, trước đây định
	// nghĩa xong nhưng KHÔNG BAO GIỜ được đăng ký ở đâu cả (dead code), khiến
	// model không có cách nào liệt kê những gì đã "nhớ" (cùng loại thiếu sót
	// đã fix cho rag.list trước đây).
	for name, names := range map[string]map[string]bool{
		"code": codeTools, "research": researchTools, "general": generalTools,
	} {
		if !names["memory.save"] || !names["memory.recall"] || !names["memory.list"] {
			t.Errorf("%s registry thiếu tool memory: %v", name, names)
		}
	}
}

// TestBuildRegistries_MemoryToolsShareStoreWithRecallPipeline khoá đúng fix:
// trước đây memory.save/recall/list dùng 1 kho HOÀN TOÀN TÁCH BIỆT
// (globalMemoryStore trong internal/tools) với *memory.Store mà
// RecallNode/ExtractNode/Learner dùng để tự động bơm "[BỘ NHỚ]" vào system
// prompt — model chủ động gọi memory.save để "nhớ giúp" điều gì đó, nhưng
// lượt sau RecallNode không hề thấy nó.
//
// Test này dựng registry với 1 *memory.Store thật (không phải fake), gọi
// memory.save qua tool, rồi xác nhận CHÍNH *memory.Store đó (không phải qua
// tool memory.recall) đã thấy dữ liệu — chứng minh 2 nơi dùng chung 1 kho.
func TestBuildRegistries_MemoryToolsShareStoreWithRecallPipeline(t *testing.T) {
	cfg := config.Config{AllowedPaths: []string{t.TempDir()}}
	store := memory.NewStore()
	general, _, _ := buildRegistries(cfg, store)
	_ = general

	code, _, _ := buildRegistries(cfg, store)
	saveTool, ok := code.Get("memory.save")
	if !ok {
		t.Fatal("registry thiếu memory.save")
	}

	args, _ := json.Marshal(map[string]string{"key": "user_name", "value": "Linh"})
	if _, err := saveTool.Execute(context.Background(), args); err != nil {
		t.Fatalf("memory.save Execute: %v", err)
	}

	// Đọc TRỰC TIẾP từ *memory.Store (không qua tool memory.recall) — đây
	// chính là kho RecallNode sẽ đọc ở lượt sau.
	if v, found := store.Get("default", "user_name"); !found || v != "Linh" {
		t.Fatalf("store.Get(default,user_name) = (%q,%v), want (Linh,true) — memory.save phải ghi vào CÙNG Store với RecallNode", v, found)
	}
}

// Bug: filter.go's FilterToolDefs hứa web.search/web.fetch cho query có
// hasCodeIntent (vd từ khoá "search" nằm trong codeKeywords), nhưng trước fix
// codeRegistry không có 2 tool này → agent code lỗi runtime "tool not found".
// registerRAGAndCodeExtras phải cấp web.search/web.fetch cho codeRegistry.
func TestRegisterRAGAndCodeExtras_CodeGetsWebTools(t *testing.T) {
	cfg := config.Config{AllowedPaths: []string{t.TempDir()}}
	code, research, general := buildRegistries(cfg, memory.NewStore())

	ragTool := tools.NewRAGSearchTool(nil, "db", "", nil, tools.RAGSearchConfig{})
	ragReadTool := tools.NewRAGReadTool(nil, "db")
	ragListTool := tools.NewRAGListTool(nil, "db")
	registerRAGAndCodeExtras(code, research, general, ragTool, ragReadTool, ragListTool)

	codeTools := toolNames(t, code.ToolDefs())
	for _, want := range []string{"web.search", "web.fetch", "rag.search", "rag.read", "rag.list"} {
		if !codeTools[want] {
			t.Errorf("code registry thiếu %q sau registerRAGAndCodeExtras (có: %v)", want, codeTools)
		}
	}

	researchTools := toolNames(t, research.ToolDefs())
	generalTools := toolNames(t, general.ToolDefs())
	for name, names := range map[string]map[string]bool{
		"research": researchTools, "general": generalTools,
	} {
		// rag.list phải có ở CẢ 3 agent: câu hỏi "tôi có tài liệu gì" có thể tới
		// bất kỳ agent nào tuỳ routing.
		for _, want := range []string{"rag.search", "rag.read", "rag.list"} {
			if !names[want] {
				t.Errorf("%s registry thiếu %q sau registerRAGAndCodeExtras (có: %v)", name, want, names)
			}
		}
	}
}

func TestNewHTTPHandler_Routes(t *testing.T) {
	h := newHTTPHandler(provider.NewFake(), &stubRunner{text: `["a"]`}, nil, nil, nil, nil, nil, "")

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
	h := newHTTPHandler(provider.NewFake(), &stubRunner{}, nil, nil, nil, nil, nil, "")

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
	h := newHTTPHandler(provider.NewFake(), &stubRunner{}, mongoPinger(nil), nil, nil, nil, nil, "")

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

// learnerOrNil phải trả về interface nil ĐÚNG NGHĨA khi Learner chưa được
// khởi tạo (ENABLE_LEARNER=false) — cùng lớp bug với mongoPinger: gán thẳng
// *memory.Learner nil vào interface ConversationLearner sẽ tạo ra 1 interface
// non-nil (type != nil, value == nil), khiến `if learner != nil` trong
// newHTTPHandler đánh giá sai và ChatHandler tưởng learner đã bật (P2 fix).
func TestLearnerOrNil(t *testing.T) {
	if got := learnerOrNil(nil); got != nil {
		t.Fatalf("learnerOrNil(nil) = %#v, want interface nil đúng nghĩa", got)
	}

	l := memory.NewLearner(memory.NewStore(), nil, provider.NewFake(), "model", nil)
	if got := learnerOrNil(l); got == nil {
		t.Fatal("learnerOrNil(non-nil Learner) không được trả nil")
	}
}

// Khi ENABLE_LEARNER=false (mặc định), newHTTPHandler phải nhận learner=nil
// — ChatHandler không được gọi LearnFromConversation, tránh tốn 1 LLM call
// ngoài ý muốn khi cờ tắt (P2 fix).
func TestNewHTTPHandler_LearnerDisabledByDefault(t *testing.T) {
	h := newHTTPHandler(provider.NewFake(), &stubRunner{text: "hi"}, nil, learnerOrNil(nil), nil, nil, nil, "")

	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"userMessage":"xin chào"}`)
	req := httptest.NewRequest(http.MethodPost, "/chat", body)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (chat vẫn chạy bình thường dù learner tắt)", rec.Code)
	}
}
