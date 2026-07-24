package agent

import (
	"strings"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// ClassifyTask analyzes user input to determine if thinking mode should be enabled.
// Returns the recommended thinking level: OFF for simple, LOW for medium, MEDIUM for complex.
//
// Rules:
//   - Simple (OFF): greetings, chat, translate, basic questions, short messages
//   - Medium (LOW): tool calls expected, multi-step, coding, analysis
//   - Complex (MEDIUM): research, debugging, architecture, long context, many tools
func ClassifyTask(input string, hasToolCalls bool, stepCount int) provider.ThinkingLevel {
	lower := strings.ToLower(input)

	// Complex tasks → MEDIUM thinking
	complexKeywords := []string{
		"research", "nghiên cứu", "analyze", "phân tích",
		"architecture", "kiến trúc", "design", "thiết kế",
		"debug", "sửa lỗi", "refactor", "optimize", "tối ưu",
	}
	for _, kw := range complexKeywords {
		if strings.Contains(lower, kw) {
			return provider.ThinkingMedium
		}
	}

	// Medium tasks → LOW thinking (tool calls expected)
	mediumKeywords := []string{
		"code", "review", "explain", "giải thích",
		"compare", "so sánh", "implement", "build", "tạo",
		"search", "tìm", "file", "document", "tài liệu",
		"task", "công việc", "how to", "làm sao", "cách",
		"write", "viết", "create", "update", "delete",
	}
	for _, kw := range mediumKeywords {
		if strings.Contains(lower, kw) {
			return provider.ThinkingLow
		}
	}

	// If already used tools → keep thinking on
	if hasToolCalls || stepCount > 2 {
		return provider.ThinkingLow
	}

	// Short simple messages → OFF
	if len(input) < 30 {
		return provider.ThinkingOff
	}

	// Default: OFF (fast, cheap)
	return provider.ThinkingOff
}

// DynamicThinkingConfig controls the dynamic thinking behavior.
type DynamicThinkingConfig struct {
	Enabled    bool // false = use static thinking level from config
	DefaultOff bool // true = default OFF, only enable for complex tasks
}

// ResolveThinking determines the thinking level based on config and task analysis.
// If dynamic thinking is disabled, returns the static level from config.
func ResolveThinking(cfg DynamicThinkingConfig, staticLevel provider.ThinkingLevel, input string, hasToolCalls bool, stepCount int) provider.ThinkingLevel {
	if !cfg.Enabled {
		return staticLevel
	}

	classified := ClassifyTask(input, hasToolCalls, stepCount)

	// If DefaultOff: only enable thinking for non-OFF classifications
	if cfg.DefaultOff && classified == provider.ThinkingOff {
		return provider.ThinkingOff
	}

	// If already classified as higher → use it
	if classified != provider.ThinkingOff {
		return classified
	}

	// Default with dynamic: OFF unless task needs it
	if cfg.DefaultOff {
		return provider.ThinkingOff
	}

	return provider.ThinkingLow
}
