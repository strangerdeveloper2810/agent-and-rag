package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/agent"
	"github.com/ai-agent-tut/agent-go/internal/middleware"
)

// suggestionCategories — PHẢI khớp đúng 5 category id ở FE
// (apps/web/src/modules/chat/components/EmptyState.tsx PROMPT_CATEGORY_IDS)
// để tab đang chọn lọc đúng theo category LLM gắn cho từng gợi ý.
const suggestionCategories = "creative, rag, dev, search, productivity"

// maxRecentMessages/maxFacts giới hạn ngữ cảnh đưa vào prompt — đủ để cá
// nhân hoá mà không kéo dài prompt/tốn token vô ích.
const (
	maxRecentMessages = 5
	maxFacts          = 5
)

// RecentMessagesFetcher lấy vài tin nhắn gần nhất của 1 tenant để cá nhân hoá
// gợi ý — tách interface để test không cần Mongo thật. *mongo.Client hiện
// thực interface này qua RecentUserMessages (xem internal/mongo/conversations.go).
type RecentMessagesFetcher interface {
	RecentUserMessages(ctx context.Context, tenantID string, limit int) ([]string, error)
}

// FactsProvider trả toàn bộ fact đã học của 1 tenant — *memory.Store hiện
// thực interface này qua Store.All.
type FactsProvider interface {
	All(tenantID string) map[string]string
}

// suggestionItem là 1 gợi ý kèm category để FE lọc theo tab đang chọn mà
// không cần gọi lại LLM mỗi lần đổi tab. Category rỗng ("") nghĩa là gợi ý
// này hợp lệ ở MỌI tab (dùng cho fallback tĩnh/gợi ý không phân loại được).
type suggestionItem struct {
	Text     string `json:"text"`
	Category string `json:"category,omitempty"`
}

// SuggestionsHandler generates dynamic, contextual suggestions via LLM.
type SuggestionsHandler struct {
	runner   agent.Runner
	messages RecentMessagesFetcher
	facts    FactsProvider
}

// NewSuggestionsHandler creates a handler for GET /suggestions. messages/facts
// có thể nil (vd Mongo/Store chưa cấu hình) — handler bỏ qua phần ngữ cảnh
// tương ứng, không lỗi.
func NewSuggestionsHandler(runner agent.Runner, messages RecentMessagesFetcher, facts FactsProvider) *SuggestionsHandler {
	return &SuggestionsHandler{runner: runner, messages: messages, facts: facts}
}

// ServeHTTP generates suggestions by asking the LLM what would be helpful
// given the current time, recent conversation topics, and facts learned
// about THIS tenant — trước đây prompt hoàn toàn tĩnh (không đổi giữa user
// hay thời điểm), khiến gợi ý luôn quanh quẩn vài chủ đề giống nhau.
func (h *SuggestionsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := middleware.GetTenantID(ctx)

	prompt := h.buildPrompt(ctx, tenantID)

	input := agent.RunInput{
		UserMessage: prompt,
		MaxSteps:    1, // single turn, no tools needed
	}

	var fullText string
	emit := func(e agent.Event) {
		if e.Type == "text" {
			fullText += e.Text
		}
	}

	_, err := h.runner.Run(ctx, input, emit)
	if err != nil {
		writeSuggestions(w, http.StatusOK, fallbackSuggestions())
		return
	}

	suggestions := parseSuggestions(fullText)
	if len(suggestions) == 0 {
		suggestions = fallbackSuggestions()
	}
	if len(suggestions) > 8 {
		suggestions = suggestions[:8]
	}
	writeSuggestions(w, http.StatusOK, suggestions)
}

// buildPrompt dựng prompt cá nhân hoá theo tenant: thời gian thật + lịch sử
// hội thoại gần đây + facts đã học. Mỗi phần bị bỏ qua êm nếu không lấy được
// (Mongo lỗi, chưa có Store, user mới chưa có gì) — KHÔNG chặn request.
func (h *SuggestionsHandler) buildPrompt(ctx context.Context, tenantID string) string {
	var b strings.Builder
	b.WriteString("Bạn là JARVIS. Người dùng vừa mở ứng dụng chat.\n\n")

	now := time.Now()
	b.WriteString("Hôm nay là " + weekdayVN(now) + ", " + timeOfDayVN(now) + ".\n")

	if h.messages != nil {
		recent, err := h.messages.RecentUserMessages(ctx, tenantID, maxRecentMessages)
		if err != nil {
			slog.Warn("suggestions: không lấy được lịch sử hội thoại gần đây", "err", err)
		}
		if len(recent) > 0 {
			b.WriteString("\nCác câu hỏi/chủ đề gần đây của người dùng này:\n")
			for _, m := range recent {
				b.WriteString("- " + truncateRunes(m, 100) + "\n")
			}
		}
	}

	if h.facts != nil {
		facts := h.facts.All(tenantID)
		if len(facts) > 0 {
			b.WriteString("\nMột số điều đã biết về người dùng này:\n")
			count := 0
			for k, v := range facts {
				if count >= maxFacts {
					break
				}
				b.WriteString("- " + k + ": " + truncateRunes(v, 100) + "\n")
				count++
			}
		}
	}

	b.WriteString(`
Dựa vào ngữ cảnh trên (nếu có) và khả năng của bạn (tìm kiếm web, đọc/ghi
file, quản lý task, ghi nhớ thông tin, nghiên cứu, phân tích code, thời
tiết, tính toán, dịch thuật, lịch, ghi chú, timer...), hãy đề xuất 8 câu hỏi
hoặc tác vụ mà người dùng CÓ THỂ muốn làm TIẾP THEO — ưu tiên liên quan tới
ngữ cảnh cụ thể của người dùng này (nếu có) hơn là câu hỏi chung chung, và
KHÔNG lặp lại nguyên văn các câu hỏi gần đây đã liệt kê trên.

QUY TẮC:
- Trả lời CHỈ 1 mảng JSON, không giải thích gì thêm.
- Mỗi phần tử là {"text": "...", "category": "..."}.
- "text": 1 câu ngắn gọn (dưới 80 ký tự), VIẾT BẰNG TIẾNG VIỆT.
- "category": PHẢI là 1 trong: ` + suggestionCategories + `.
- Đa dạng category, không dồn hết vào 1 category.
- Format: [{"text": "...", "category": "dev"}, ...]`)

	return b.String()
}

func writeSuggestions(w http.ResponseWriter, code int, suggestions []suggestionItem) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{
		"suggestions": suggestions,
	})
}

// parseSuggestions parse response LLM — ưu tiên format mới [{"text","category"}],
// nhưng vẫn chấp nhận format cũ ["câu 1", "câu 2"] (category rỗng) để không vỡ
// nếu model lệch hướng dẫn. Cũng dò tìm mảng JSON trong text nếu model lỡ kèm
// giải thích/markdown quanh mảng.
func parseSuggestions(text string) []suggestionItem {
	raw := extractJSONArray(text)
	if raw == "" {
		return nil
	}

	var tagged []suggestionItem
	if err := json.Unmarshal([]byte(raw), &tagged); err == nil {
		return tagged
	}

	var flat []string
	if err := json.Unmarshal([]byte(raw), &flat); err == nil {
		out := make([]suggestionItem, 0, len(flat))
		for _, s := range flat {
			if s != "" {
				out = append(out, suggestionItem{Text: s})
			}
		}
		return out
	}

	return nil
}

// extractJSONArray tìm đoạn "[...]" trong text (model đôi khi kèm giải thích
// hoặc markdown code fence quanh mảng JSON dù đã được yêu cầu không làm vậy).
func extractJSONArray(text string) string {
	start := strings.IndexByte(text, '[')
	end := strings.LastIndexByte(text, ']')
	if start < 0 || end <= start {
		return ""
	}
	return text[start : end+1]
}

func fallbackSuggestions() []suggestionItem {
	return []suggestionItem{
		{Text: "Hôm nay có việc gì cần làm không?"},
		{Text: "Tìm tài liệu về dự án gần đây"},
		{Text: "Giải thích cách hoạt động của RAG"},
		{Text: "Tạo task mới cho cuộc họp sắp tới"},
		{Text: "Nghiên cứu về AI agent architecture"},
		{Text: "Dịch đoạn văn sang tiếng Anh"},
	}
}

// weekdayVN/timeOfDayVN: Go không có locale tiếng Việt sẵn cho time.Weekday,
// nên tự map. Tách hàm pure để test không cần fake "đồng hồ".
func weekdayVN(t time.Time) string {
	names := map[time.Weekday]string{
		time.Monday:    "Thứ Hai",
		time.Tuesday:   "Thứ Ba",
		time.Wednesday: "Thứ Tư",
		time.Thursday:  "Thứ Năm",
		time.Friday:    "Thứ Sáu",
		time.Saturday:  "Thứ Bảy",
		time.Sunday:    "Chủ Nhật",
	}
	return names[t.Weekday()]
}

func timeOfDayVN(t time.Time) string {
	switch h := t.Hour(); {
	case h < 5:
		return "đêm khuya"
	case h < 11:
		return "buổi sáng"
	case h < 13:
		return "buổi trưa"
	case h < 18:
		return "buổi chiều"
	default:
		return "buổi tối"
	}
}

// truncateRunes cắt chuỗi theo RUNE (không theo byte) để không chẻ giữa ký
// tự tiếng Việt đa byte.
func truncateRunes(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "…"
}
