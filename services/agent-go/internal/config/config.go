// Package config nạp cấu hình từ biến môi trường (fail-fast nếu thiếu bắt buộc).
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config chứa toàn bộ cấu hình JARVIS — từ LLM provider đến storage paths.
type Config struct {
	// Server
	Port string // default: 3002

	// Provider (LLM)
	Provider             string // "gemini" | "anthropic" | "deepseek" | "ollama" | "openai_compat" | "auto"
	GeminiKey            string
	GeminiModel          string
	GeminiSecondaryModel string   // fallback model khi primary bị rate limit (vd: "gemini-3.5-flash-lite")
	GeminiFallbackModels []string // danh sách các model Gemini fallback theo thứ tự ưu tiên
	ThinkingLevel        string   // OFF|LOW|MEDIUM|HIGH (Gemini 3.x)
	AnthropicKey         string
	AnthropicModel       string
	AnthropicKey2        string // Claude key thứ 2, dùng làm fallback tiếp theo nếu key 1 hết quota (optional)
	DeepSeekKey          string
	DeepSeekFlashModel   string // cho task đơn giản, rẻ + nhanh
	DeepSeekProModel     string // cho task cần reasoning nhiều

	// ReflectionBatchTurns: số lượt chat gộp lại trước khi Learner thực sự
	// chạy 1 lần reflection (giảm số request LLM nền). default: 3.
	ReflectionBatchTurns int

	// Ollama (local LLM)
	OllamaURL   string // default: http://localhost:11434
	OllamaModel string // default: llama3.1:8b

	// OpenAI-compatible local server (vLLM, llama.cpp server, LM Studio...).
	// Không có default cố định như Ollama vì mỗi server nghe ở cổng khác nhau
	// và chạy model do người dùng tự nạp — phải cấu hình rõ qua env.
	OpenAICompatBaseURL string // OPENAI_COMPAT_BASE_URL, vd: http://localhost:8000/v1
	OpenAICompatModel   string // OPENAI_COMPAT_MODEL
	OpenAICompatKey     string // OPENAI_COMPAT_API_KEY, optional — nhiều server local không cần auth

	// SQLite Storage
	DBPath       string   // SQLite database path. default: jarvis.db
	SkillsDir    string   // Skills directory. default: ./skills
	AllowedPaths []string // File tool allowed paths. default: [".", "$HOME"]

	// MongoDB (optional — dùng chung với apps/api, cho RAG documents + tasks)
	MongoURI string // MONGODB_URI. Để trống = không dùng Mongo
	MongoDB  string // default: ai_agent_tut

	// Embedding
	VoyageKey  string
	EmbedModel string // embedding model. default: nomic-embed-text

	// RAG
	EnableHybridSearch bool
	EnableRerank       bool // rerank MIỄN PHÍ (keyword overlap, không gọi LLM). default: true

	// Parent Document Retrieval: mở rộng mỗi kết quả rag.search bằng các chunk
	// liền kề (chunkIndex ±1) cùng tài liệu để LLM có thêm ngữ cảnh xung quanh
	// đoạn khớp. Chỉ tốn thêm 1 Mongo query, KHÔNG gọi LLM — an toàn để bật
	// mặc định.
	EnableParentRetrieval bool

	// LLM Rerank: chấm điểm lại thứ tự kết quả rag.search bằng 1 lời gọi LLM
	// (khác rerankKeyword miễn phí ở trên). TẮT mặc định vì tốn thêm 1 LLM
	// call mỗi lần rag.search — khi bật, ưu tiên hơn EnableRerank (keyword).
	EnableLLMRerank bool

	// HyDE (Hypothetical Document Embeddings): trước khi embed câu hỏi, gọi
	// LLM sinh 1 đoạn trả lời giả định rồi embed đoạn đó thay vì câu hỏi thô
	// (thường gần nghĩa với đoạn văn thật hơn). TẮT mặc định vì tốn thêm 1
	// LLM call mỗi lần rag.search.
	EnableHyDE bool

	// LangSmith Tracing
	LangSmithAPIKey   string
	LangSmithProject  string
	LangSmithEndpoint string
	LangSmithTracing  bool

	// Limits
	MaxSteps int // default: 12
	// MaxTokens giới hạn output token cho MỖI lần gọi LLM của luồng chat chính.
	// Trước đây field này là CONFIG CHẾT (khai báo = 0 nhưng không nơi nào đọc),
	// nên request thật gửi max_tokens=0 → API dùng mặc định của nó, không có
	// trần nào. Log dev thật: một câu trả lời 15.858 ký tự / 4.747 token mất 44
	// giây. 0 = không giới hạn; đặt qua MAX_OUTPUT_TOKENS.
	MaxTokens int
	// MaxContextTokens là ngân sách token context trước khi trimContext kích
	// hoạt (Tier 2) — cũng chính là ContextBudget gửi cho FE qua event done để
	// tự gợi ý bắt đầu chat mới khi context lớn (Tier 4). Trước đây field này
	// là CONFIG CHẾT tương tự MaxTokens ở trên: hardcode 100000 khi khởi tạo
	// (không đọc MAX_CONTEXT_TOKENS), và main.go không hề gọi
	// Engine.SetMaxContextTokens(cfg.MaxContextTokens) — Engine luôn dùng
	// default nội bộ của riêng nó (cũng 100000, nên trùng hợp "chạy đúng"
	// nhưng đặt biến môi trường khác đi không có tác dụng gì). 0 = không giới
	// hạn (trimContext không bao giờ kích hoạt).
	MaxContextTokens int
	MaxToolOutput    int // max chars from tool output MỖI tool call. default: 24000
	// MaxTotalToolOutput giới hạn TỔNG số ký tự tool output cộng dồn qua TẤT CẢ
	// bước của một lượt chạy (không phải từng tool call riêng lẻ).
	//
	// Vì sao cần thêm, dù đã có MaxToolOutput: log dev thật cho thấy 1 lượt chat
	// đạt 46.542 input token ở step 4 — không phải vì 1 tool call nào vượt trần
	// riêng, mà vì NHIỀU tool call (rag.search, notes.search...) cộng dồn qua 4
	// step, và mỗi step gửi lại TOÀN BỘ lịch sử message nên nội dung tool cũ bị
	// trả tiền lặp lại ở mọi step sau. MaxToolOutput chặn được 1 tool "nói quá
	// nhiều", nhưng không chặn được nhiều tool "nói vừa phải" cộng dồn thành quá
	// nhiều. Đặt qua MAX_TOTAL_TOOL_OUTPUT, 0 = không giới hạn.
	MaxTotalToolOutput int
	ShellTimeout       int // seconds. default: 30
	// ShellAllowedCommands là allowlist tên lệnh cho phép shell.exec chạy, đọc từ
	// SHELL_ALLOWED_COMMANDS (phân tách bằng dấu phẩy, xem splitCSV). Để trống →
	// dùng defaultShellAllowedCommands bên dưới.
	//
	// Vì sao cần: trước đây cả 3 call site production (buildRegistries "code" /
	// "general", promptsize probe) truyền thẳng nil cho NewShellToolWithTimeout,
	// tức shell.exec cho phép chạy BẤT KỲ lệnh nào — allowlist đã viết sẵn trong
	// shellTool nhưng không nơi nào thực sự bật nó lên. Khi JARVIS chỉ chạy 1
	// người trên máy cá nhân thì chấp nhận được; mở cho nhiều người dùng thì đây
	// là remote code execution trần trụi.
	ShellAllowedCommands  []string
	EnableDynamicThinking bool // auto-adjust thinking level based on task complexity
	EnablePlanning        bool // LLM plan node cho task phức tạp. default: false (tiết kiệm 1 LLM call trước token đầu)

	// Autonomous continuous learning / memory reflection worker.
	// TẮT mặc định — mỗi response tốn thêm 1 LLM call (~1500 max token) chạy
	// nền để trích xuất user facts + knowledge items. Bật qua ENABLE_LEARNER=true
	// khi cần tính năng "học liên tục"; để tắt trong dev/test nhằm tiết kiệm
	// chi phí + tránh side-effect ghi Mongo ngoài ý muốn.
	EnableLearner bool

	// AllowDestructiveTools cho phép agent TỰ CHẠY tool xếp loại
	// KindDestructive (hiện chỉ có shell.exec) mà không cần xác nhận.
	//
	// TẮT MẶC ĐỊNH vì đây là quyền chạy lệnh shell tuỳ ý trên máy người dùng.
	// Khi tắt, guardrails chặn tool và agent trả về thông báo giải thích kèm
	// hướng dẫn (xem node_tools.destructiveBlockedMessage) thay vì câu trả lời
	// rỗng như trước. Chỉ bật trên máy cá nhân, khi người dùng chủ động muốn.
	AllowDestructiveTools bool

	// OwnerTenantIDs là danh sách tenant được coi là CHỦ HỆ THỐNG, tức được dùng
	// nhóm tool đặc quyền (file.read/write/search, shell.exec, git) — xem
	// tools.IsPrivilegedTool. Đọc từ OWNER_TENANT_IDS, phân tách bằng dấu phẩy.
	//
	// Vì sao cần: nhóm tool này tác động lên MÁY CHẠY AGENT chứ không phải máy
	// người dùng, và không scope theo tenant. Với 1 người trên máy cá nhân thì
	// vô hại; khi mở cho nhiều người dùng thì bất kỳ ai cũng đọc được .env chứa
	// API key của server. Để rỗng = chỉ tenant "default" (chạy local không auth)
	// có đặc quyền (fail closed).
	OwnerTenantIDs []string
}

// defaultMaxOutputTokens là trần output token mặc định cho luồng chat chính.
//
// Chọn 8192: đủ rộng cho câu trả lời dài kèm code block (log thật cho thấy câu
// dài nhất ~4.700 token), nhưng vẫn chặn được trường hợp model viết tràn lan
// hàng chục nghìn token khiến người dùng chờ 40+ giây. Khi chạm trần, provider
// trả finish_reason=length → engine set s.Truncated → UI hiện chỉ báo + nút
// "Tiếp tục" (hạ tầng này đã có sẵn nhưng trước đây không bao giờ kích hoạt vì
// không có trần nào).
const defaultMaxOutputTokens = 8192

// defaultMaxTotalToolOutput là ngân sách TỔNG mặc định cho tool output cộng dồn
// qua cả lượt chạy.
//
// Chọn 60.000 ký tự (~15K token): đủ cho vài lần rag.search full-size (mỗi lần
// chạm trần MaxToolOutput=24.000 ký tự) trước khi bị siết lại, nhưng vẫn ngăn
// được kiểu tích luỹ đã thấy trong log dev thật (46.542 input token ở step 4,
// phần lớn từ tool output cộng dồn qua nhiều bước).
const defaultMaxTotalToolOutput = 60000

// defaultShellAllowedCommands là allowlist mặc định cho shell.exec khi
// SHELL_ALLOWED_COMMANDS không được cấu hình.
//
// CHỈ gồm lệnh đọc/inspect vô hại — KHÔNG có interpreter nào (node, python, go,
// npm, bash, sh...) vì bất kỳ interpreter nào cũng cho phép thực thi mã tuỳ ý,
// biến allowlist thành vô nghĩa (vd: "python -c 'import os; os.system(...)'").
// Operator nào thật sự cần mở rộng thì tự set SHELL_ALLOWED_COMMANDS — mặc định
// phải an toàn theo kiểu fail-closed, không phải "tiện cho dev".
var defaultShellAllowedCommands = []string{
	"git", "ls", "grep", "cat", "find", "pwd", "echo", "wc", "head", "tail", "diff",
}

// Load đọc .env (nếu có) rồi env → Config với defaults hợp lý.
// Không fail nếu không có .env — chỉ dùng biến môi trường có sẵn.
func Load() (Config, error) {
	// Tự động load .env từ current directory (không lỗi nếu không tồn tại)
	_ = godotenv.Load()

	langSmithAPIKey := envOr("LANGSMITH_API_KEY", os.Getenv("LANGCHAIN_API_KEY"))
	langSmithProject := envOr("LANGSMITH_PROJECT", envOr("LANGCHAIN_PROJECT", "ai-agent-tut"))
	langSmithEndpoint := envOr("LANGSMITH_ENDPOINT", envOr("LANGCHAIN_ENDPOINT", "https://api.smith.langchain.com"))
	langSmithTracing := envOr("LANGSMITH_TRACING", envOr("LANGCHAIN_TRACING_V2", "false")) == "true"
	if langSmithAPIKey != "" && (os.Getenv("LANGSMITH_TRACING") == "" && os.Getenv("LANGCHAIN_TRACING_V2") == "") {
		langSmithTracing = true
	}

	shellAllowedCommands := splitCSV(os.Getenv("SHELL_ALLOWED_COMMANDS"))
	if len(shellAllowedCommands) == 0 {
		shellAllowedCommands = defaultShellAllowedCommands
	}

	c := Config{
		Port:                 envOr("PORT", "3002"),
		Provider:             envOr("LLM_PROVIDER", "gemini"),
		GeminiKey:            envOr("GEMINI_API_KEY", os.Getenv("GOOGLE_API_KEY")),
		GeminiModel:          envOr("GEMINI_MODEL", "gemini-3.1-flash-lite"),
		GeminiSecondaryModel: envOr("GEMINI_SECONDARY_MODEL", "gemini-3.5-flash-lite"),
		// gemini-2.0-flash/gemini-2.0-flash-lite ĐÃ BỊ GOOGLE KHAI TỬ (API trả
		// 404 NOT_FOUND) — production log cho thấy chain fallback vẫn chạy
		// đúng default hard-code này (không phải danh sách cấu hình trong env
		// production), tốn 2 lượt gọi chắc chắn thất bại mỗi khi rơi xuống
		// cuối chain (quota Gemini free-tier cạn, việc xảy ra hàng ngày).
		GeminiFallbackModels: splitCSV(envOr("GEMINI_FALLBACK_MODELS", "gemini-3.5-flash-lite,gemini-3.7-flash,gemini-3.6-flash,gemini-3.5-flash,gemini-2.5-flash-lite,gemini-2.5-flash")),

		ThinkingLevel:         envOr("GOOGLE_THINKING_LEVEL", "OFF"),
		AnthropicKey:          os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicModel:        envOr("CLAUDE_MODEL", "claude-haiku-4-5-20251001"),
		AnthropicKey2:         os.Getenv("ANTHROPIC_API_KEY_2"),
		OllamaURL:             envOr("OLLAMA_URL", "http://localhost:11434"),
		OllamaModel:           envOr("OLLAMA_MODEL", "llama3.1:8b"),
		OpenAICompatBaseURL:   os.Getenv("OPENAI_COMPAT_BASE_URL"),
		OpenAICompatModel:     os.Getenv("OPENAI_COMPAT_MODEL"),
		OpenAICompatKey:       os.Getenv("OPENAI_COMPAT_API_KEY"),
		DeepSeekKey:           os.Getenv("DEEPSEEK_API_KEY"),
		DeepSeekFlashModel:    envOr("DEEPSEEK_FLASH_MODEL", "deepseek-v4-flash"),
		DeepSeekProModel:      envOr("DEEPSEEK_PRO_MODEL", "deepseek-v4-pro"),
		ReflectionBatchTurns:  intEnvOr("REFLECTION_BATCH_TURNS", 3),
		DBPath:                envOr("JARVIS_DB_PATH", "jarvis.db"),
		SkillsDir:             envOr("JARVIS_SKILLS_DIR", "./skills"),
		AllowedPaths:          []string{".", os.Getenv("HOME")},
		MongoURI:              os.Getenv("MONGODB_URI"),
		MongoDB:               envOr("MONGODB_DB", "ai_agent_tut"),
		VoyageKey:             os.Getenv("VOYAGE_API_KEY"),
		EmbedModel:            envOr("EMBED_MODEL", "nomic-embed-text"),
		EnableHybridSearch:    envOr("ENABLE_HYBRID_SEARCH", "true") == "true",
		EnableRerank:          envOr("ENABLE_RERANK", "true") == "true",
		EnableParentRetrieval: envOr("ENABLE_PARENT_RETRIEVAL", "true") == "true",
		EnableLLMRerank:       envOr("ENABLE_LLM_RERANK", "false") == "true",
		EnableHyDE:            envOr("ENABLE_HYDE", "false") == "true",
		LangSmithAPIKey:       langSmithAPIKey,
		LangSmithProject:      langSmithProject,
		LangSmithEndpoint:     langSmithEndpoint,
		LangSmithTracing:      langSmithTracing,
		MaxSteps:              12,
		MaxTokens:             intEnvOr("MAX_OUTPUT_TOKENS", defaultMaxOutputTokens),
		MaxContextTokens:      intEnvOr("MAX_CONTEXT_TOKENS", 100000),
		MaxToolOutput:         24000,
		MaxTotalToolOutput:    intEnvOr("MAX_TOTAL_TOOL_OUTPUT", defaultMaxTotalToolOutput),
		ShellTimeout:          30,
		ShellAllowedCommands:  shellAllowedCommands,
		EnableDynamicThinking: envOr("ENABLE_DYNAMIC_THINKING", "false") == "true",
		EnablePlanning:        envOr("ENABLE_PLANNING", "false") == "true",
		AllowDestructiveTools: envOr("ALLOW_DESTRUCTIVE_TOOLS", "false") == "true",
		OwnerTenantIDs:        splitCSV(os.Getenv("OWNER_TENANT_IDS")),
		EnableLearner:         envOr("ENABLE_LEARNER", "false") == "true",
	}

	// Validate: ít nhất 1 LLM provider phải được cấu hình
	hasProvider := false
	switch c.Provider {
	case "gemini":
		hasProvider = c.GeminiKey != ""
	case "anthropic":
		hasProvider = c.AnthropicKey != ""
	case "deepseek":
		hasProvider = c.DeepSeekKey != ""
	case "ollama":
		hasProvider = true // local, no key needed
	case "openai_compat":
		hasProvider = true // local server, key optional (nhiều server không cần auth)
	case "auto":
		hasProvider = c.GeminiKey != "" || c.AnthropicKey != "" || c.DeepSeekKey != "" // cần ít nhất 1 key
	default:
		return Config{}, fmt.Errorf("unknown LLM_PROVIDER: %q (use gemini, anthropic, deepseek, ollama, openai_compat, or auto)", c.Provider)
	}
	if !hasProvider {
		return Config{}, fmt.Errorf("%s requires API key (set GEMINI_API_KEY or ANTHROPIC_API_KEY)", c.Provider)
	}

	return c, nil
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// splitCSV tách chuỗi phân tách bằng dấu phẩy, bỏ khoảng trắng và phần tử rỗng.
func splitCSV(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// intEnvOr đọc biến môi trường dạng số nguyên. Giá trị không parse được hoặc âm
// → dùng def (fail-safe, không làm sập server vì một biến gõ sai).
func intEnvOr(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < 0 {
		slog.Warn("config: biến môi trường không phải số nguyên hợp lệ, dùng mặc định",
			"key", k, "value", v, "default", def)
		return def
	}
	return n
}
