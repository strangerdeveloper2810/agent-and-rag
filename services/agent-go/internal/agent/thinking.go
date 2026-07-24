package agent

import (
	"strings"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// ClassifyTask analyzes user input to determine if thinking mode should be enabled.
// Returns OFF for simple, LOW for medium, MEDIUM for complex.
func ClassifyTask(input string, hasToolCalls bool, stepCount int) provider.ThinkingLevel {
	lower := strings.ToLower(input)

	// Complex tasks → MEDIUM thinking
	complexKeywords := []string{
		"research", "nghiên cứu", "analyze", "phân tích",
		"architecture", "kiến trúc", "design", "thiết kế",
		"debug", "sửa lỗi", "refactor", "optimize", "tối ưu",
		"tranh chấp", "pháp lý", "luật", "sở hữu",
		"hợp đồng", "khiếu nại", "kiện", "tòa án",
		"thủ tục", "quy định", "nghị định", "thông tư",
		"bảo mật", "security", "vulnerability", "cve",
	}
	for _, kw := range complexKeywords {
		if strings.Contains(lower, kw) {
			return provider.ThinkingMedium
		}
	}

	// Medium tasks → LOW thinking
	mediumKeywords := []string{
		"code", "review", "explain", "giải thích",
		"compare", "so sánh", "implement", "build", "tạo",
		"search", "tìm", "file", "document", "tài liệu",
		"task", "công việc", "how to", "làm sao", "cách",
		"write", "viết", "create", "update", "delete",
		"xuất", "export", "lưu", "save",
		"dịch", "translate", "tóm tắt", "summarize",
	}
	for _, kw := range mediumKeywords {
		if strings.Contains(lower, kw) {
			return provider.ThinkingLow
		}
	}

	// If already used tools in previous turns → keep thinking on
	if hasToolCalls || stepCount > 2 {
		return provider.ThinkingLow
	}

	// Long messages (>200 chars) likely need deeper analysis
	if len(input) > 200 {
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
	Enabled    bool
	DefaultOff bool // true = default OFF, only enable for complex tasks
}

func ResolveThinking(cfg DynamicThinkingConfig, staticLevel provider.ThinkingLevel, input string, hasToolCalls bool, stepCount int) provider.ThinkingLevel {
	if !cfg.Enabled {
		return staticLevel
	}

	classified := ClassifyTask(input, hasToolCalls, stepCount)

	if cfg.DefaultOff {
		return classified // DefaultOff: use classified level directly
	}

	if classified != provider.ThinkingOff {
		return classified
	}

	return provider.ThinkingLow
}
