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
	"github.com/ai-agent-tut/agent-go/internal/observability"
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

	// --- Wire Observability & LangSmith Tracer ---
	lsClient := observability.InitLangSmith(cfg)
	defer lsClient.Close()

	// --- Wire Provider ---
	prov, err := factory.New(cfg)
	if err != nil {
		slog.Error("provider", "err", err)
		os.Exit(1)
	}

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

	// --- Wire Memory Store ---
	// Tạo TRƯỚC buildRegistries: tool memory.save/recall/list (bên trong
	// buildRegistries) và pipeline recall/extract/learn tự động phải dùng
	// CHUNG 1 Store — trước đây 2 kho tách biệt hoàn toàn (xem comment tại
	// tools.NewSaveMemoryTool).
	store := memory.NewStore()

	// Nạp lại fact đã học từ lần chạy trước (collection "memories", ghi bởi
	// Learner.saveFactToMongo) — trước fix này, Store chỉ sống trong RAM nên
	// mọi thứ agent "học" được biến mất sau mỗi lần restart/deploy dù đã ghi
	// bền xuống Mongo (kho bền chỉ để ghi, không ai đọc lại). Không có Mongo
	// → no-op, không chặn khởi động.
	if mongoClient != nil {
		loadCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		n, err := store.LoadFromMongo(loadCtx, mongoClient)
		cancel()
		if err != nil {
			slog.Warn("memory: nạp fact từ mongo thất bại — bắt đầu với Store rỗng", "err", err)
		} else {
			slog.Info("memory: đã nạp fact từ mongo", "count", n)
		}
	}

	// --- Wire Scoped Tool Registries per Agent Specialty ---
	codeRegistry, researchRegistry, generalRegistry := buildRegistries(cfg, store)

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
	ragListTool := tools.NewRAGListTool(mongoClient, cfg.MongoDB)
	registerRAGAndCodeExtras(codeRegistry, researchRegistry, generalRegistry, ragTool, ragReadTool, ragListTool)

	// Nhóm tool đặc quyền (file.*, shell.exec, git) tác động lên MÁY CHẠY AGENT và
	// không scope theo tenant → chỉ chủ hệ thống được dùng. Log rõ trạng thái để
	// không ai vô tình mở nó cho người lạ.
	if len(cfg.OwnerTenantIDs) == 0 {
		slog.Warn("guardrails: OWNER_TENANT_IDS chưa cấu hình — tool đặc quyền CHỈ dùng được ở chế độ local (tenant \"default\")",
			"privileged_tools", tools.PrivilegedToolNames(),
			"hint", "đặt OWNER_TENANT_IDS=<tenant id của bạn> trong .env nếu muốn dùng nhóm tool này khi đã đăng nhập")
	} else {
		slog.Info("guardrails: tool đặc quyền giới hạn theo chủ hệ thống",
			"owner_tenants", len(cfg.OwnerTenantIDs),
			"privileged_tools", tools.PrivilegedToolNames())
	}

	// --- Wire Circuit Breaker ---
	cb := guardrails.NewCircuitBreaker(3)

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

	// lang="vi" cứng ở đây vì BuildSystemPrompt được gọi MỘT LẦN lúc wiring,
	// còn Engine (và system prompt của nó) được CHIA SẺ giữa mọi request đồng
	// thời của mọi user — xem comment trong orchestrator.Register() giải
	// thích tại sao KHÔNG được gọi SetSystemPrompt mỗi request (data race).
	// Lựa chọn ngôn ngữ theo user (FE gửi field "lang" trong ChatRequest) được
	// áp dụng RIÊNG cho từng lượt chạy qua RunInput.Lang → State.Lang, và
	// nodeModel() ghi đè chỉ dẫn ngôn ngữ một cách an toàn (per-request, không
	// đụng vào systemPrompt dùng chung) — xem node_model.go.
	generalEngine := agent.NewEngine(prov, generalRegistry)
	generalEngine.SetSystemPrompt(agent.BuildSystemPrompt(nil, skillSummaries, "vi"))
	generalEngine.SetDynamicThinking(dynThinking)
	generalEngine.SetCircuitBreaker(cb)
	generalEngine.SetMaxToolOutput(cfg.MaxToolOutput)
	generalEngine.SetMaxTotalToolOutput(cfg.MaxTotalToolOutput)
	generalEngine.SetMaxOutputTokens(cfg.MaxTokens)
	generalEngine.SetOwnerTenants(cfg.OwnerTenantIDs)
	generalEngine.SetAllowDestructiveTools(cfg.AllowDestructiveTools)
	generalEngine.SetSkillLoader(skillLoader)
	generalEngine.SetMaxContextTokens(cfg.MaxContextTokens)
	generalEngine.SetFastModel(fastModel(cfg))
	generalEngine.SetMemoryNodes(
		memory.RecallNode(store),
		memory.ExtractNode(store),
		memory.SummarizeNode(prov, fastModel(cfg)),
	)
	if planningEnabled {
		generalEngine.EnablePlanning()
	}

	codeEngine := agent.NewEngine(prov, codeRegistry)
	codeEngine.SetSystemPrompt(agent.BuildSystemPrompt(nil, skillSummaries, "vi"))
	codeEngine.SetDynamicThinking(dynThinking)
	codeEngine.SetCircuitBreaker(cb)
	codeEngine.SetMaxToolOutput(cfg.MaxToolOutput)
	codeEngine.SetMaxTotalToolOutput(cfg.MaxTotalToolOutput)
	codeEngine.SetMaxOutputTokens(cfg.MaxTokens)
	codeEngine.SetOwnerTenants(cfg.OwnerTenantIDs)
	codeEngine.SetAllowDestructiveTools(cfg.AllowDestructiveTools)
	codeEngine.SetSkillLoader(skillLoader)
	codeEngine.SetMaxContextTokens(cfg.MaxContextTokens)
	codeEngine.SetFastModel(fastModel(cfg))
	codeEngine.SetMemoryNodes(
		memory.RecallNode(store),
		memory.ExtractNode(store),
		memory.SummarizeNode(prov, fastModel(cfg)),
	)
	if planningEnabled {
		codeEngine.EnablePlanning()
	}

	researchEngine := agent.NewEngine(prov, researchRegistry)
	researchEngine.SetSystemPrompt(agent.BuildSystemPrompt(nil, skillSummaries, "vi"))
	researchEngine.SetDynamicThinking(dynThinking)
	researchEngine.SetCircuitBreaker(cb)
	researchEngine.SetMaxToolOutput(cfg.MaxToolOutput)
	researchEngine.SetMaxTotalToolOutput(cfg.MaxTotalToolOutput)
	researchEngine.SetMaxOutputTokens(cfg.MaxTokens)
	researchEngine.SetOwnerTenants(cfg.OwnerTenantIDs)
	researchEngine.SetAllowDestructiveTools(cfg.AllowDestructiveTools)
	researchEngine.SetSkillLoader(skillLoader)
	researchEngine.SetMaxContextTokens(cfg.MaxContextTokens)
	researchEngine.SetFastModel(fastModel(cfg))
	researchEngine.SetMemoryNodes(
		memory.RecallNode(store),
		memory.ExtractNode(store),
		memory.SummarizeNode(prov, fastModel(cfg)),
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
		SystemPrompt:    agent.BuildSystemPrompt(nil, skillSummaries, "vi"),
	})
	orch.Register(&orchestrator.AgentSpec{
		Name:        "code",
		Description: "Code and programming specialist for development tasks",
		Engine:      codeEngine,
		// Trigger keyword khớp theo word boundary (xem orchestrator.matchTrigger)
		// nên "go" không còn kéo theo "golang"/"mongo"/"django". Danh sách cũ chỉ
		// có 12 từ tiếng Anh, thiếu toàn bộ thuật ngữ FE và tiếng Việt, nên câu
		// hỏi kiểu "Viết custom hook useMemo với useSelector" rơi hết về agent
		// general (registry không có git/version).
		TriggerKeywords: []string{
			"code", "coding", "programming", "function", "func", "bug", "debug",
			"go", "golang", "python", "typescript", "javascript", "rust", "java",
			"refactor", "test", "unit test", "compile", "build", "deploy",
			"react", "hook", "hooks", "component", "redux", "vue", "angular",
			"nestjs", "nodejs", "node", "express", "fastify", "css", "tailwind",
			"html", "sql", "query", "api", "endpoint", "struct", "interface",
			"class", "method", "regex", "docker", "kubernetes",
			"viết hàm", "viết code", "sửa lỗi", "lỗi", "hàm", "biến", "thư viện",
			"triển khai", "tối ưu", "mã nguồn",
		},
		SystemPrompt: agent.BuildSystemPrompt(nil, skillSummaries, "vi"),
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
		SystemPrompt: agent.BuildSystemPrompt(nil, skillSummaries, "vi") + `
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
//
// store dùng CHUNG cho memory.save/recall/list và pipeline recall/extract/
// learn tự động (RecallNode/ExtractNode/Learner) — xem comment tại
// tools.NewSaveMemoryTool cho lý do tại sao 2 nơi này PHẢI cùng 1 Store.
func buildRegistries(cfg config.Config, store *memory.Store) (code, research, general *tools.Registry) {
	code = tools.NewRegistry()
	code.Register(tools.NewFileSearchTool(cfg.AllowedPaths))
	code.Register(tools.NewFileReadTool(cfg.AllowedPaths))
	code.Register(tools.NewFileWriteTool(cfg.AllowedPaths))
	code.Register(tools.NewShellToolWithTimeout(nil, time.Duration(cfg.ShellTimeout)*time.Second))
	code.Register(tools.NewGitTool("."))
	code.Register(tools.NewVersionTool())
	code.Register(tools.NewSaveMemoryTool(store))
	code.Register(tools.NewRecallMemoryTool(store))
	code.Register(tools.NewListMemoriesTool(store))

	research = tools.NewRegistry()
	research.Register(tools.NewWebSearchTool(nil))
	research.Register(tools.NewWebFetchTool(nil))
	research.Register(tools.NewNotesSearchTool("."))
	research.Register(tools.NewNotesCreateTool("."))
	research.Register(tools.NewSaveMemoryTool(store))
	research.Register(tools.NewRecallMemoryTool(store))
	research.Register(tools.NewListMemoriesTool(store))

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
	general.Register(tools.NewSaveMemoryTool(store))
	general.Register(tools.NewRecallMemoryTool(store))
	general.Register(tools.NewListMemoriesTool(store))

	return code, research, general
}

// registerRAGAndCodeExtras wires rag.search/rag.read vào cả 3 registry, và cấp
// thêm web.search/web.fetch cho codeRegistry.
// Tách khỏi main để test được: filter.go's FilterToolDefs hứa web.search/web.fetch
// cho query có hasCodeIntent (vd từ khoá "search" nằm trong codeKeywords), nên
// codeRegistry PHẢI thật sự có 2 tool này, nếu không agent code sẽ gặp lỗi
// runtime "tool not found: web.search".
func registerRAGAndCodeExtras(codeRegistry, researchRegistry, generalRegistry *tools.Registry, ragTool, ragReadTool, ragListTool tools.Tool) {
	for _, reg := range []*tools.Registry{codeRegistry, researchRegistry, generalRegistry} {
		reg.Register(ragTool)
		reg.Register(ragReadTool)
		reg.Register(ragListTool)
	}

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
