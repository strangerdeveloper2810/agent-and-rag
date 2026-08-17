package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// UserFact represents a learned preference, technology choice, or biographical fact about the user.
type UserFact struct {
	Category   string  `json:"category"`   // e.g. "tech_stack", "coding_preference", "user_profile", "rule"
	Key        string  `json:"key"`        // e.g. "backend_framework", "css_preference", "user_name"
	Value      string  `json:"value"`      // e.g. "Go + Fastify", "Vanilla CSS only"
	Confidence float64 `json:"confidence"` // 0.0 to 1.0
}

// KnowledgeItem represents an extracted solution, debug lesson, or best practice.
type KnowledgeItem struct {
	Title   string   `json:"title"`   // e.g. "Sửa lỗi Google Search scraping bằng Tavily"
	Summary string   `json:"summary"` // short 1-2 sentence description
	Tags    []string `json:"tags"`    // e.g. ["web-search", "tavily", "scraping"]
	Content string   `json:"content"` // Markdown content detailing the solution or best practice
}

// ReflectionResult is the structured output from the LLM reflection pass.
type ReflectionResult struct {
	UserFacts      []UserFact      `json:"user_facts"`
	KnowledgeItems []KnowledgeItem `json:"knowledge_items"`
}

const reflectionSystemPrompt = `Bạn là hệ thống Trích Xuất & Học Tri Thức Tự Động (Autonomous Knowledge & Memory Learner) cho AI Assistant J.A.R.V.I.S.
Nhiệm vụ của bạn là phân tích đoạn hội thoại vừa diễn ra giữa Người dùng (User) và Trợ lý (Assistant) để trích xuất:

1. "user_facts": Các thông tin, sở thích, tech stack, quy ước làm việc hoặc luật mới mà người dùng đề cập (chỉ lấy thông tin rõ ràng và có ích cho các phiên sau).
   - Category: "tech_stack" | "coding_preference" | "user_profile" | "rule"
   - Key: tên định danh ngắn gọn bằng tiếng Anh (vd: "web_framework", "css_style", "user_role")
   - Value: giá trị chi tiết
   - Confidence: độ tin cậy từ 0.7 đến 1.0

2. "knowledge_items": Các bài học kinh nghiệm, giải pháp kỹ thuật vừa sửa lỗi thành công, hoặc quy chuẩn kiến trúc quan trọng được giải quyết trong hội thoại (chỉ tạo khi có vấn đề kỹ thuật hoặc bài học thực sự có giá trị).
   - Title: Tiêu đề rõ ràng
   - Summary: Tóm tắt 1-2 câu
   - Tags: Mảng các từ khóa liên quan
   - Content: Nội dung chi tiết bằng Markdown giải thích vấn đề và cách giải quyết

BẮT BUỘC trả về định dạng JSON thuần túy (không kèm markdown code block hoặc text giải thích):
{
  "user_facts": [
    {"category": "tech_stack", "key": "...", "value": "...", "confidence": 0.95}
  ],
  "knowledge_items": [
    {"title": "...", "summary": "...", "tags": ["..."], "content": "..."}
  ]
}
Nếu không có thông tin hay bài học nào mới đáng nhớ, hãy trả về:
{"user_facts": [], "knowledge_items": []}`

// ReflectAndExtract runs a fast LLM pass over conversation messages to extract user facts and knowledge items.
func ReflectAndExtract(ctx context.Context, p provider.Provider, model string, messages []provider.Message) (*ReflectionResult, error) {
	if len(messages) == 0 || p == nil {
		return &ReflectionResult{}, nil
	}

	// Format conversation transcript for reflection
	var convText strings.Builder
	for _, m := range messages {
		if m.Role != provider.RoleUser && m.Role != provider.RoleAssistant {
			continue
		}
		convText.WriteString(fmt.Sprintf("%s: %s\n\n", strings.ToUpper(string(m.Role)), m.Content))
	}

	trimmedConv := convText.String()
	if len(trimmedConv) > 8000 {
		trimmedConv = trimmedConv[len(trimmedConv)-8000:]
	}

	req := provider.GenerateRequest{
		System: reflectionSystemPrompt,
		Messages: []provider.Message{
			{
				Role:    provider.RoleUser,
				Content: fmt.Sprintf("Hãy phân tích đoạn hội thoại sau và trích xuất tri thức / sở thích / bài học mới:\n\n%s", trimmedConv),
			},
		},
		Options: provider.ProviderOptions{
			Model:     model,
			MaxTokens: 1500,
		},
	}

	chunkChan, err := p.Generate(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("reflection generate error: %w", err)
	}

	var fullResp strings.Builder
	for chunk := range chunkChan {
		if chunk.Kind == provider.ChunkText {
			fullResp.WriteString(chunk.Text)
		}
	}

	raw := strings.TrimSpace(fullResp.String())
	// Strip markdown code fence if LLM wrapped in ```json
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var res ReflectionResult
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		slog.Warn("memory: failed to parse reflection json", "err", err, "raw", raw)
		return &ReflectionResult{}, nil
	}

	return &res, nil
}
