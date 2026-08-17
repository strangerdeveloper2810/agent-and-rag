package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// errReflectionParseFailed đánh dấu lỗi parse JSON (khác lỗi gọi LLM thật sự
// như network/API) để ReflectAndExtract biết khi nào nên retry.
var errReflectionParseFailed = errors.New("reflection: không parse được JSON trả về")

// maxReflectionAttempts: LLM đôi khi sinh JSON sai cú pháp (vd quên escape
// dấu ngoặc kép bên trong 1 string value — xem repairTruncatedJSON cho case
// bị cắt cụt riêng). Đây là lỗi xác suất, thử lại 1 lần thường ra kết quả
// hợp lệ thay vì mất trắng lượt học đó.
const maxReflectionAttempts = 2

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
   - Content: Nội dung chi tiết bằng Markdown giải thích vấn đề và cách giải quyết — SÚC TÍCH, tối đa khoảng 300 từ (đủ ý chính, không cần đầy đủ như tài liệu)

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
{"user_facts": [], "knowledge_items": []}

QUAN TRỌNG về escaping JSON: mọi dấu ngoặc kép (") xuất hiện BÊN TRONG một
giá trị chuỗi (vd trong "content") BẮT BUỘC phải escape thành \". Khi cần
trích dẫn giá trị dạng chuỗi/config (ví dụ similarity: "cosine"), ưu tiên
dùng dấu nháy đơn ' thay vì nháy kép để tránh phá vỡ cấu trúc JSON. Toàn bộ
output phải là JSON hợp lệ, parse được bằng json.Unmarshal ngay lần đầu.`

// ReflectAndExtract runs a fast LLM pass over conversation messages to extract user facts and knowledge items.
// Lỗi parse JSON (khác lỗi Generate() thật sự) được retry tối đa
// maxReflectionAttempts lần trước khi bỏ cuộc — đây là lỗi xác suất phụ
// thuộc vào output cụ thể của model, không phải lỗi hệ thống, nên thử lại
// thường cứu được lượt học thay vì mất trắng.
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

	var lastErr error
	for attempt := 1; attempt <= maxReflectionAttempts; attempt++ {
		res, err := reflectOnce(ctx, p, model, trimmedConv)
		if err == nil {
			return res, nil
		}
		if !errors.Is(err, errReflectionParseFailed) {
			// Lỗi Generate() thật sự (network/API) — không phải lỗi xác
			// suất của 1 lần sinh, retry không giúp ích, trả lỗi luôn.
			return nil, err
		}
		lastErr = err
		slog.Warn("memory: reflection JSON parse thất bại, thử lại", "attempt", attempt, "max_attempts", maxReflectionAttempts, "err", err)
	}

	slog.Warn("memory: reflection JSON vẫn không parse được sau khi thử lại — bỏ qua lượt học này", "err", lastErr)
	return &ReflectionResult{}, nil
}

// reflectOnce chạy đúng 1 lượt Generate() + parse JSON. Trả về
// errReflectionParseFailed (wrap) khi lỗi là do JSON sai cú pháp — caller
// (ReflectAndExtract) dựa vào đây để quyết định retry.
func reflectOnce(ctx context.Context, p provider.Provider, model, trimmedConv string) (*ReflectionResult, error) {
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
			MaxTokens: 4096,
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
		// Response bị cắt cụt (chạm MaxTokens giữa chừng, thường ngay trong
		// string "content" dài của knowledge_items) khiến JSON không hợp lệ.
		// json.Unmarshal là all-or-nothing nên nếu bỏ qua thẳng, TOÀN BỘ kết
		// quả — kể cả user_facts đã hoàn chỉnh TRƯỚC chỗ bị cắt — cũng mất
		// theo. Thử "vá" JSON (đóng string/bracket còn dang dở) để cứu được
		// phần dữ liệu hoàn chỉnh trước khi bị cắt.
		if repaired := repairTruncatedJSON(raw); repaired != raw {
			if err2 := json.Unmarshal([]byte(repaired), &res); err2 == nil {
				slog.Warn("memory: reflection JSON bị cắt cụt (chạm MaxTokens) — đã khôi phục phần chưa cắt", "original_err", err)
				return &res, nil
			}
		}
		return nil, fmt.Errorf("%w: %v (raw=%q)", errReflectionParseFailed, err, raw)
	}

	return &res, nil
}

// repairTruncatedJSON cố gắng biến 1 chuỗi JSON bị cắt cụt giữa chừng thành
// hợp lệ, bằng cách đóng string literal còn dang dở (nếu có) rồi đóng mọi
// "{"/"[" chưa khớp theo đúng thứ tự LIFO. Không parse lại nội dung bên trong
// — chỉ "vá" phần đuôi bị thiếu, nên field bị cắt (vd content) có thể kết
// thúc giữa câu, nhưng các field hoàn chỉnh trước đó (vd user_facts) được
// giữ nguyên thay vì mất trắng toàn bộ.
func repairTruncatedJSON(raw string) string {
	var stack []byte
	inString := false
	escaped := false

	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch c {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{', '[':
			stack = append(stack, c)
		case '}', ']':
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}

	if !inString && len(stack) == 0 {
		return raw // đã hợp lệ (hoặc lỗi không phải do cắt cụt) — không sửa gì
	}

	var b strings.Builder
	b.WriteString(raw)
	if inString {
		b.WriteByte('"')
	}
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i] == '{' {
			b.WriteByte('}')
		} else {
			b.WriteByte(']')
		}
	}
	return b.String()
}
