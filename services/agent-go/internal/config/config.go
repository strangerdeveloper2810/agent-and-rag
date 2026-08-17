// Package config nạp cấu hình từ biến môi trường (fail-fast nếu thiếu bắt buộc).
package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Config chứa toàn bộ cấu hình JARVIS — từ LLM provider đến storage paths.
type Config struct {
	// Server
	Port string // default: 3002

	// Provider (LLM)
	Provider           string // "gemini" | "anthropic" | "ollama" | "deepseek"
	GeminiKey          string
	GeminiModel        string
	ThinkingLevel      string // OFF|LOW|MEDIUM|HIGH (Gemini 3.x)
	AnthropicKey       string
	AnthropicModel     string
	DeepSeekKey        string
	DeepSeekFlashModel string // cho task đơn giản, rẻ + nhanh
	DeepSeekProModel   string // cho task cần reasoning nhiều

	// Ollama (local LLM)
	OllamaURL   string // default: http://localhost:11434
	OllamaModel string // default: llama3.1:8b

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

	// Limits
	MaxSteps              int  // default: 12
	MaxTokens             int  // default: 0 (unlimited output tokens)
	MaxContextTokens      int  // max context tokens before trimming. default: 100000
	MaxToolOutput         int  // max chars from tool output. default: 24000
	ShellTimeout          int  // seconds. default: 30
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
}

// Load đọc .env (nếu có) rồi env → Config với defaults hợp lý.
// Không fail nếu không có .env — chỉ dùng biến môi trường có sẵn.
func Load() (Config, error) {
	// Tự động load .env từ current directory (không lỗi nếu không tồn tại)
	_ = godotenv.Load()

	c := Config{
		Port:                  envOr("PORT", "3002"),
		Provider:              envOr("LLM_PROVIDER", "gemini"),
		GeminiKey:             envOr("GEMINI_API_KEY", os.Getenv("GOOGLE_API_KEY")),
		GeminiModel:           envOr("GEMINI_MODEL", "gemini-2.5-flash"),
		ThinkingLevel:         envOr("GOOGLE_THINKING_LEVEL", "OFF"),
		AnthropicKey:          os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicModel:        envOr("CLAUDE_MODEL", "claude-haiku-4-5-20251001"),
		OllamaURL:             envOr("OLLAMA_URL", "http://localhost:11434"),
		OllamaModel:           envOr("OLLAMA_MODEL", "llama3.1:8b"),
		DeepSeekKey:           os.Getenv("DEEPSEEK_API_KEY"),
		DeepSeekFlashModel:    envOr("DEEPSEEK_FLASH_MODEL", "deepseek-v4-flash"),
		DeepSeekProModel:      envOr("DEEPSEEK_PRO_MODEL", "deepseek-v4-pro"),
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
		MaxSteps:              12,
		MaxTokens:             0,
		MaxContextTokens:      100000,
		MaxToolOutput:         24000,
		ShellTimeout:          30,
		EnableDynamicThinking: envOr("ENABLE_DYNAMIC_THINKING", "false") == "true",
		EnablePlanning:        envOr("ENABLE_PLANNING", "false") == "true",
		AllowDestructiveTools: envOr("ALLOW_DESTRUCTIVE_TOOLS", "false") == "true",
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
	case "auto":
		hasProvider = c.GeminiKey != "" || c.AnthropicKey != "" || c.DeepSeekKey != "" // cần ít nhất 1 key
	default:
		return Config{}, fmt.Errorf("unknown LLM_PROVIDER: %q (use gemini, anthropic, deepseek, ollama, or auto)", c.Provider)
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
