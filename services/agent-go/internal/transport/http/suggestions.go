package http

import (
	"encoding/json"
	"net/http"

	"github.com/ai-agent-tut/agent-go/internal/agent"
)

// SuggestionsHandler generates dynamic, contextual suggestions via LLM.
type SuggestionsHandler struct {
	runner agent.Runner
}

// NewSuggestionsHandler creates a handler for GET /suggestions.
func NewSuggestionsHandler(runner agent.Runner) *SuggestionsHandler {
	return &SuggestionsHandler{runner: runner}
}

// ServeHTTP generates suggestions by asking the LLM what would be helpful
// given the current time, available skills, and tools.
func (h *SuggestionsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Build a special one-shot prompt that asks for suggestions
	prompt := `Bạn là JARVIS. Người dùng vừa mở ứng dụng chat.

Dựa vào thời gian hiện tại, khả năng của bạn (tìm kiếm web, đọc/ghi file,
quản lý task, ghi nhớ thông tin, nghiên cứu, phân tích code, thời tiết,
tính toán, dịch thuật, lịch, ghi chú, timer...), hãy đề xuất 6 câu hỏi
hoặc tác vụ mà người dùng CÓ THỂ muốn làm.

QUY TẮC:
- Trả lời CHỈ 1 mảng JSON string, không giải thích gì thêm.
- 6 suggestions, mỗi cái 1 câu ngắn gọn (dưới 80 ký tự).
- Đa dạng: 2 câu về công việc hằng ngày, 2 câu về kỹ thuật/code,
  1 câu sáng tạo, 1 câu về kiến thức.
- VIẾT BẰNG TIẾNG VIỆT.
- Format: ["suggestion 1", "suggestion 2", ...]`

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

	_, err := h.runner.Run(r.Context(), input, emit)
	if err != nil {
		// Fallback: return hardcoded suggestions if LLM fails
		writeSuggestions(w, http.StatusOK, fallbackSuggestions())
		return
	}

	// Try to parse the response as JSON array
	var suggestions []string
	if err := json.Unmarshal([]byte(fullText), &suggestions); err != nil {
		// LLM returned non-JSON — try to extract array from text
		suggestions = extractSuggestions(fullText)
	}

	if len(suggestions) == 0 {
		suggestions = fallbackSuggestions()
	}

	writeSuggestions(w, http.StatusOK, suggestions[:min(6, len(suggestions))])
}

func writeSuggestions(w http.ResponseWriter, code int, suggestions []string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{
		"suggestions": suggestions,
	})
}

// extractSuggestions tries to find a JSON array in the LLM response text.
func extractSuggestions(text string) []string {
	// Find [...] in text
	start := -1
	end := -1
	for i, c := range text {
		if c == '[' && start == -1 {
			start = i
		}
		if c == ']' {
			end = i + 1
		}
	}
	if start >= 0 && end > start {
		var suggestions []string
		if err := json.Unmarshal([]byte(text[start:end]), &suggestions); err == nil {
			return suggestions
		}
	}
	return nil
}

func fallbackSuggestions() []string {
	return []string{
		"Hôm nay có việc gì cần làm không?",
		"Tìm tài liệu về dự án gần đây",
		"Giải thích cách hoạt động của RAG",
		"Tạo task mới cho cuộc họp sắp tới",
		"Nghiên cứu về AI agent architecture",
		"Dịch đoạn văn sang tiếng Anh",
	}
}
