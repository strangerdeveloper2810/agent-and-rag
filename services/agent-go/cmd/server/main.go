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
	"github.com/ai-agent-tut/agent-go/internal/guardrails"
	"github.com/ai-agent-tut/agent-go/internal/memory"
	"github.com/ai-agent-tut/agent-go/internal/middleware"
	"github.com/ai-agent-tut/agent-go/internal/mongo"
	"github.com/ai-agent-tut/agent-go/internal/orchestrator"
	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/provider/factory"
	"github.com/ai-agent-tut/agent-go/internal/provider/ollama"
	"github.com/ai-agent-tut/agent-go/internal/rag"
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

	// --- Wire Scoped Tool Registries per Agent Specialty ---
	codeRegistry, researchRegistry, generalRegistry := buildRegistries(cfg)

	// --- Wire MongoDB (optional — for RAG document search) ---
	var mongoClient *mongo.Client
	if cfg.MongoURI != "" {
		mc, err := mongo.Connect(context.Background(), cfg.MongoURI, cfg.MongoDB)
		if err != nil {
			slog.Warn("mongo: connection failed, RAG disabled", "err", err)
		} else {
			mongoClient = mc
			slog.Info("mongo: connected", "db", cfg.MongoDB)
			defer func() {
				_ = mongoClient.Close(context.Background())
			}()
		}
	}
	// RAG tools (register to all registries)
	ragTool := tools.NewRAGSearchTool(mongoClient, cfg.MongoDB, cfg.VoyageKey, prov, tools.RAGSearchConfig{
		EnableHybridSearch:    cfg.EnableHybridSearch,
		EnableRerank:          cfg.EnableRerank,
		EnableLLMRerank:       cfg.EnableLLMRerank,
		EnableParentRetrieval: cfg.EnableParentRetrieval,
		EnableHyDE:            cfg.EnableHyDE,
		Model:                 fastModel(cfg),
	})
	ragReadTool := tools.NewRAGReadTool(mongoClient, cfg.MongoDB)
	registerRAGAndCodeExtras(codeRegistry, researchRegistry, generalRegistry, ragTool, ragReadTool)

	// --- Wire Circuit Breaker ---
	cb := guardrails.NewCircuitBreaker(3)

	// --- Wire Memory Store ---
	store := memory.NewStore()

	// Wire embedding provider for semantic memory recall.
	if cfg.VoyageKey != "" {
		vc := rag.NewClient(cfg.VoyageKey)
		store.SetEmbedder(memory.EmbedderFunc(func(ctx context.Context, texts []string) ([][]float64, error) {
			return vc.Embed(ctx, texts, "document")
		}))
		slog.Info("memory: semantic embedding enabled", "provider", "voyage")
	} else if cfg.Provider == "ollama" || os.Getenv("OLLAMA_URL") != "" {
		embedClient, err := ollama.New(cfg.OllamaURL, cfg.EmbedModel)
		if err != nil {
			slog.Warn("memory: ollama embed client creation failed", "err", err)
		} else {
			store.SetEmbedder(memory.EmbedderFunc(func(ctx context.Context, texts []string) ([][]float64, error) {
				vecs32, err := embedClient.Embed(ctx, texts)
				if err != nil {
					return nil, err
				}
				result := make([][]float64, len(vecs32))
				for i, v := range vecs32 {
					result[i] = make([]float64, len(v))
					for j, val := range v {
						result[i][j] = float64(val)
					}
				}
				return result, nil
			}))
			slog.Info("memory: semantic embedding enabled", "provider", "ollama", "url", cfg.OllamaURL)
		}
	}

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
	dynThinking := agent.DynamicThinkingConfig{Enabled: cfg.EnableDynamicThinking, DefaultOff: true}

	// Planning (plan/reflect LLM nodes) TẮT mặc định — bật qua ENABLE_PLANNING=true.
	// Khi bật, request phức tạp tốn thêm 1 LLM call trước token đầu tiên.
	planningEnabled := cfg.EnablePlanning

	generalEngine := agent.NewEngine(prov, generalRegistry)
	generalEngine.SetSystemPrompt(agent.BuildSystemPrompt(nil, skillSummaries))
	generalEngine.SetDynamicThinking(dynThinking)
	generalEngine.SetCircuitBreaker(cb)
	generalEngine.SetSkillLoader(skillLoader)
	generalEngine.SetMemoryNodes(
		memory.RecallNode(store),
		memory.ExtractNode(store),
		memory.SummarizeNode(),
	)
	if planningEnabled {
		generalEngine.EnablePlanning()
	}

	codeEngine := agent.NewEngine(prov, codeRegistry)
	codeEngine.SetSystemPrompt(agent.BuildSystemPrompt(nil, skillSummaries))
	codeEngine.SetDynamicThinking(dynThinking)
	codeEngine.SetCircuitBreaker(cb)
	codeEngine.SetSkillLoader(skillLoader)
	codeEngine.SetMemoryNodes(
		memory.RecallNode(store),
		memory.ExtractNode(store),
		memory.SummarizeNode(),
	)
	if planningEnabled {
		codeEngine.EnablePlanning()
	}

	researchEngine := agent.NewEngine(prov, researchRegistry)
	researchEngine.SetSystemPrompt(agent.BuildSystemPrompt(nil, skillSummaries))
	researchEngine.SetDynamicThinking(dynThinking)
	researchEngine.SetCircuitBreaker(cb)
	researchEngine.SetSkillLoader(skillLoader)
	researchEngine.SetMemoryNodes(
		memory.RecallNode(store),
		memory.ExtractNode(store),
		memory.SummarizeNode(),
	)
	if planningEnabled {
		researchEngine.EnablePlanning()
	}

	orch := orchestrator.New()
	orch.Register(&orchestrator.AgentSpec{
		Name:            "general",
		Description:     "General-purpose assistant for everyday tasks and conversation",
		Engine:          generalEngine,
		TriggerKeywords: []string{},
		SystemPrompt:    agent.BuildSystemPrompt(nil, skillSummaries),
	})
	orch.Register(&orchestrator.AgentSpec{
		Name:        "code",
		Description: "Code and programming specialist for development tasks",
		Engine:      codeEngine,
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

	// --- Wire Autonomous Learner (opt-in via ENABLE_LEARNER — costs 1 extra
	// LLM call per response, TẮT mặc định) ---
	var learner *memory.Learner
	if cfg.EnableLearner {
		var embedder memory.Embedder
		if cfg.VoyageKey != "" {
			vc := rag.NewClient(cfg.VoyageKey)
			embedder = memory.EmbedderFunc(func(ctx context.Context, texts []string) ([][]float64, error) {
				return vc.Embed(ctx, texts, "document")
			})
		}
		learner = memory.NewLearner(store, mongoClient, prov, fastModel(cfg), embedder)
		slog.Info("learner: autonomous continuous learning enabled")
	}

	// --- HTTP Routes ---
	srv := &http.Server{Addr: ":" + cfg.Port, Handler: newHTTPHandler(prov, orch, mongoPinger(mongoClient), learnerOrNil(learner))}

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

// buildRegistries dựng 3 registry tool theo chuyên môn agent (code, research,
// general). Tách khỏi main để test được danh sách tool của từng agent.
func buildRegistries(cfg config.Config) (code, research, general *tools.Registry) {
	code = tools.NewRegistry()
	code.Register(tools.NewFileSearchTool(cfg.AllowedPaths))
	code.Register(tools.NewFileReadTool(cfg.AllowedPaths))
	code.Register(tools.NewFileWriteTool(cfg.AllowedPaths))
	code.Register(tools.NewShellToolWithTimeout(nil, time.Duration(cfg.ShellTimeout)*time.Second))
	code.Register(tools.NewGitTool("."))
	code.Register(tools.NewVersionTool())
	code.Register(tools.NewSaveMemoryTool())
	code.Register(tools.NewRecallMemoryTool())

	research = tools.NewRegistry()
	research.Register(tools.NewWebSearchTool(nil))
	research.Register(tools.NewWebFetchTool(nil))
	research.Register(tools.NewNotesSearchTool("."))
	research.Register(tools.NewNotesCreateTool("."))
	research.Register(tools.NewSaveMemoryTool())
	research.Register(tools.NewRecallMemoryTool())

	general = tools.NewRegistry()
	general.Register(tools.NewEchoTool())
	general.Register(tools.NewFileSearchTool(cfg.AllowedPaths))
	general.Register(tools.NewFileReadTool(cfg.AllowedPaths))
	general.Register(tools.NewFileWriteTool(cfg.AllowedPaths))
	general.Register(tools.NewShellToolWithTimeout(nil, time.Duration(cfg.ShellTimeout)*time.Second))
	general.Register(tools.NewWebSearchTool(nil))
	general.Register(tools.NewWebFetchTool(nil))
	general.Register(tools.NewTranslateTool(nil))
	general.Register(tools.NewCalculatorTool())
	general.Register(tools.NewDateTimeTool())
	general.Register(tools.NewSaveMemoryTool())
	general.Register(tools.NewRecallMemoryTool())

	return code, research, general
}

// registerRAGAndCodeExtras wires rag.search/rag.read vào cả 3 registry, và cấp
// thêm web.search/web.fetch cho codeRegistry.
// Tách khỏi main để test được: filter.go's FilterToolDefs hứa web.search/web.fetch
// cho query có hasCodeIntent (vd từ khoá "search" nằm trong codeKeywords), nên
// codeRegistry PHẢI thật sự có 2 tool này, nếu không agent code sẽ gặp lỗi
// runtime "tool not found: web.search".
func registerRAGAndCodeExtras(codeRegistry, researchRegistry, generalRegistry *tools.Registry, ragTool, ragReadTool tools.Tool) {
	codeRegistry.Register(ragTool)
	codeRegistry.Register(ragReadTool)
	researchRegistry.Register(ragTool)
	researchRegistry.Register(ragReadTool)
	generalRegistry.Register(ragTool)
	generalRegistry.Register(ragReadTool)

	codeRegistry.Register(tools.NewWebSearchTool(nil))
	codeRegistry.Register(tools.NewWebFetchTool(nil))
}

// fastModel chọn model rẻ/nhanh cho các tác vụ phụ trợ tốn thêm 1 LLM call
// (reflection, LLM rerank, HyDE) — KHÔNG dùng model chính cho hội thoại để
// tránh đội chi phí. Ưu tiên DeepSeek flash (rẻ nhất) nếu có key, rơi về
// Gemini model chính nếu không.
func fastModel(cfg config.Config) string {
	if cfg.DeepSeekFlashModel != "" && cfg.DeepSeekKey != "" {
		return cfg.DeepSeekFlashModel
	}
	return cfg.GeminiModel
}

// newHTTPHandler dựng router + chuỗi middleware (CORS → Tenant → mux).
// Tách khỏi main để test được routing và middleware mà không cần chạy server.
func newHTTPHandler(prov provider.Provider, runner agent.Runner, pinger agenthttp.MongoPinger, learner agenthttp.ConversationLearner) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", agenthttp.Healthz)
	mux.HandleFunc("GET /readyz", agenthttp.NewReadyzHandler(prov, pinger))
	chatHandler := agenthttp.NewChatHandler(runner)
	if learner != nil {
		chatHandler.SetLearner(learner)
	}
	mux.HandleFunc("POST /chat", chatHandler.ServeHTTP)
	mux.HandleFunc("GET /suggestions", agenthttp.NewSuggestionsHandler(runner).ServeHTTP)

	var handler http.Handler = mux
	handler = middleware.TenantMiddleware(handler)
	handler = middleware.CORSMiddleware(handler)
	return handler
}

// mongoPinger trả interface MongoPinger, giữ nil ĐÚNG NGHĨA khi chưa nối Mongo
// (gán thẳng *mongo.Client nil vào interface sẽ tạo interface non-nil và làm
// readyz gọi Ping trên con trỏ nil).
func mongoPinger(c *mongo.Client) agenthttp.MongoPinger {
	if c == nil {
		return nil
	}
	return c
}

// learnerOrNil trả interface ConversationLearner, giữ nil ĐÚNG NGHĨA khi
// Learner chưa được khởi tạo (ENABLE_LEARNER=false) — cùng lý do với
// mongoPinger ở trên: gán thẳng *memory.Learner nil vào interface sẽ tạo
// interface non-nil và làm ChatHandler tưởng learner đã bật.
func learnerOrNil(l *memory.Learner) agenthttp.ConversationLearner {
	if l == nil {
		return nil
	}
	return l
}
