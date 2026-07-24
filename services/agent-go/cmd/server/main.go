// Command server là entrypoint của JARVIS agent runtime.
// Nạp config, wire Provider + Tool Registry + Engine, start HTTP server (SSE chat),
// graceful shutdown.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/agent"
	"github.com/ai-agent-tut/agent-go/internal/config"
	"github.com/ai-agent-tut/agent-go/internal/memory"
	"github.com/ai-agent-tut/agent-go/internal/orchestrator"
	"github.com/ai-agent-tut/agent-go/internal/provider/factory"
	"github.com/ai-agent-tut/agent-go/internal/skills"
	"github.com/ai-agent-tut/agent-go/internal/tools"
	agenthttp "github.com/ai-agent-tut/agent-go/internal/transport/http"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	// --- Wire Provider ---
	prov, err := factory.New(cfg)
	if err != nil {
		slog.Error("provider", "err", err)
		os.Exit(1)
	}

	// --- Wire Tool Registry ---
	registry := tools.NewRegistry()
	// System tools
	registry.Register(tools.NewEchoTool())
	registry.Register(tools.NewFileSearchTool(cfg.AllowedPaths))
	registry.Register(tools.NewFileReadTool(cfg.AllowedPaths))
	registry.Register(tools.NewFileWriteTool(cfg.AllowedPaths))
	registry.Register(tools.NewShellTool(nil)) // allow all commands
	// Web tools
	registry.Register(tools.NewWebSearchTool(nil))
	registry.Register(tools.NewWebFetchTool(nil))
	// Dev tools
	registry.Register(tools.NewGitTool("."))
	registry.Register(tools.NewVersionTool())
	// Personal tools
	registry.Register(tools.NewNotesSearchTool("."))
	registry.Register(tools.NewNotesCreateTool("."))
	registry.Register(tools.NewCalendarTool(""))
	registry.Register(tools.NewWeatherTool(nil))
	registry.Register(tools.NewTranslateTool(nil))
	registry.Register(tools.NewTimerTool())
	registry.Register(tools.NewHTTPTool(nil))
	// Utility tools
	registry.Register(tools.NewCalculatorTool())
	registry.Register(tools.NewDateTimeTool())
	registry.Register(tools.NewJSONTool())
	// Memory tools
	registry.Register(tools.NewSaveMemoryTool())
	registry.Register(tools.NewRecallMemoryTool())
	registry.Register(tools.NewListMemoriesTool())

	// --- Wire Memory Store ---
	store := memory.NewStore()

	// --- Wire Skills Loader ---
	skillsDir := cfg.SkillsDir
	if skillsDir == "" {
		skillsDir = "skills" // default relative to working directory
	}
	skillLoader, err := skills.NewLoader(skillsDir)
	if err != nil {
		slog.Error("skills", "err", err)
		os.Exit(1)
	}
	slog.Info("skills loaded", "count", skillLoader.Len())
	skillSummaries := skillLoader.ListSkills()

	// --- Wire Orchestrator (multi-agent) ---
	generalEngine := agent.NewEngine(prov, registry)
	generalEngine.SetSystemPrompt(agent.BuildSystemPrompt(nil, skillSummaries))
	generalEngine.SetMemoryNodes(
		memory.RecallNode(store),
		memory.ExtractNode(store),
		memory.SummarizeNode(),
	)

	codeEngine := agent.NewEngine(prov, registry)
	codeEngine.SetSystemPrompt(agent.BuildSystemPrompt(nil, skillSummaries))
	codeEngine.SetMemoryNodes(
		memory.RecallNode(store),
		memory.ExtractNode(store),
		memory.SummarizeNode(),
	)

	researchEngine := agent.NewEngine(prov, registry)
	researchEngine.SetSystemPrompt(agent.BuildSystemPrompt(nil, skillSummaries))
	researchEngine.SetMemoryNodes(
		memory.RecallNode(store),
		memory.ExtractNode(store),
		memory.SummarizeNode(),
	)

	orch := orchestrator.New()
	orch.Register(&orchestrator.AgentSpec{
		Name:            "general",
		Description:     "General-purpose assistant for everyday tasks and conversation",
		Engine:          generalEngine,
		TriggerKeywords: []string{},
		SystemPrompt:    agent.BuildSystemPrompt(nil, skillSummaries),
	})
	orch.Register(&orchestrator.AgentSpec{
		Name:            "code",
		Description:     "Code and programming specialist for development tasks",
		Engine:          codeEngine,
		TriggerKeywords: []string{"code", "programming", "function", "bug", "debug",
			"go", "python", "typescript", "javascript", "rust", "refactor", "test"},
		SystemPrompt: agent.BuildSystemPrompt(nil, skillSummaries),
	})
	orch.Register(&orchestrator.AgentSpec{
		Name:        "research",
		Description: "Deep internet research — multi-source search, cross-reference, synthesize with citations",
		Engine:      researchEngine,
		TriggerKeywords: []string{
			"search", "research", "tìm hiểu", "tra cứu", "find out", "look up",
			"what is", "who is", "when did", "how to", "latest", "tin tức",
			"news", "kiến thức", "cho biết", "giải thích", "why", "tại sao",
		},
		SystemPrompt: agent.BuildSystemPrompt(nil, skillSummaries) + `
[BẠN LÀ RESEARCH AGENT]
Bạn là chuyên gia nghiên cứu internet của JARVIS. Nhiệm vụ của bạn:

1. KHI NÀO ĐƯỢC GỌI: Khi người dùng cần thông tin ngoài kiến thức của JARVIS — tin tức mới, sự kiện gần đây, kiến thức chuyên sâu, so sánh, đánh giá.

2. QUY TRÌNH NGHIÊN CỨU:
   a. Tạo 2-3 truy vấn tìm kiếm từ các góc độ khác nhau
   b. Chạy TẤT CẢ truy vấn SONG SONG qua web.search
   c. Đọc 3-5 kết quả hàng đầu từ MỖI truy vấn qua web.fetch
   d. Đối chiếu chéo: tìm điểm đồng thuận, điểm khác biệt, điểm mâu thuẫn
   e. Tổng hợp câu trả lời bằng ngôn ngữ của bạn
   f. MỌI tuyên bố thực tế PHẢI có ít nhất 1 trích dẫn [Source: title, URL]

3. ĐỊNH DẠNG KẾT QUẢ:
   ## [Chủ đề] — Kết quả nghiên cứu
   ### Tóm tắt (1-2 câu)
   ### Phát hiện chính
   - Phát hiện 1 [Nguồn: title, URL]
   - Phát hiện 2 [Nguồn: title, URL]
   ### Thông tin mâu thuẫn (nếu có)
   ### Nguồn tham khảo
   1. [Title] — [URL]
   2. [Title] — [URL]
   ### Độ tin cậy: [Cao/Trung bình/Thấp]

4. NGUYÊN TẮC:
   - TỐI THIỂU 2 nguồn. Ưu tiên 3+.
   - Ưu tiên nguồn gần đây. Ghi rõ ngày xuất bản nếu có.
   - KHÔNG sao chép nguyên văn — TỔNG HỢP bằng từ ngữ của bạn.
   - Nếu chỉ có 1 nguồn: nói rõ "chỉ tìm thấy 1 nguồn về chủ đề này".
   - Nếu không tìm thấy gì: nói thật "Tôi đã tìm kiếm nhưng không tìm thấy thông tin đáng tin cậy về...".
   - Sau khi nghiên cứu xong: lưu phát hiện chính vào memory.save để lần sau có sẵn.

5. ĐỘ TIN CẬY NGUỒN:
   - CAO: Tài liệu chính thức, bài báo học thuật, báo chí uy tín (Reuters, AP, BBC), trang chính phủ
   - TRUNG BÌNH: Blog kỹ thuật có tác giả, Wikipedia (dùng làm điểm khởi đầu), blog công ty
   - THẤP: Blog cá nhân không rõ tác giả, diễn đàn, mạng xã hội
`,
	})
	if err := orch.SetDefault("general"); err != nil {
		slog.Error("orchestrator", "err", err)
		os.Exit(1)
	}

	// --- HTTP Routes ---
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", agenthttp.Healthz)
	mux.HandleFunc("POST /chat", agenthttp.NewChatHandler(orch).ServeHTTP)
	mux.HandleFunc("GET /suggestions", agenthttp.NewSuggestionsHandler(orch).ServeHTTP)

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: mux}

	// Start server
	go func() {
		slog.Info("jarvis listening", "addr", srv.Addr, "provider", prov.Name())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown: SIGINT/SIGTERM
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	slog.Info("shutting down…")
	_ = srv.Shutdown(ctx)
}
