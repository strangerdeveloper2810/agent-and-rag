package agent

import "github.com/ai-agent-tut/agent-go/internal/provider"

// route quyết định node kế tiếp dựa trên State hiện tại.
// Đây là hàm THUẦN (không I/O, không side-effect) → test cực dễ.
//
// Luồng quyết định (theo thứ tự ưu tiên):
//  1. Interrupt != nil          → NodeInterrupt (dừng chờ HITL)
//  2. Step >= MaxSteps          → NodeEnd (chốt an toàn)
//  3. Assistant cuối có tool calls chưa được trả lời → NodeTools
//  4. Mặc định                  → NodeEnd (final answer)
//
// "Tool calls chưa được trả lời" = assistant message cuối cùng có ít nhất 1
// ToolCall mà chưa có tool result message tương ứng (khớp ToolCallID) ở phía sau.
func route(s *State) NodeID {
	// 1. Interrupt — ưu tiên cao nhất.
	if s.Interrupt != nil {
		return NodeInterrupt
	}

	// 2. Chốt an toàn: vượt số bước tối đa (chỉ kiểm tra khi MaxSteps > 0).
	if s.MaxSteps > 0 && s.Step >= s.MaxSteps {
		return NodeEnd
	}

	// 3. Kiểm tra assistant cuối: có tool calls chưa được trả lời không?
	last := s.LastAssistant()
	if last == nil || len(last.ToolCalls) == 0 {
		return NodeEnd // không có tool call nào → final
	}

	// Đếm xem mỗi tool call đã có tool result tương ứng chưa.
	// Tool result nằm ở các message SAU assistant message này (role=tool, ToolCallID khớp).
	unanswered := countUnanswered(s, last)
	if unanswered > 0 {
		return NodeTools
	}

	// Tất cả tool calls đã có kết quả → không vào tools nữa.
	return NodeEnd
}

// countUnanswered đếm số tool calls trong lastAssistant chưa có tool result
// tương ứng trong Messages (tìm ở các message SAU lastAssistant).
func countUnanswered(s *State, last *provider.Message) int {
	// Tìm vị trí của last assistant trong Messages.
	lastIdx := -1
	for i := len(s.Messages) - 1; i >= 0; i-- {
		if &s.Messages[i] == last {
			lastIdx = i
			break
		}
	}
	if lastIdx < 0 {
		return len(last.ToolCalls) // không tìm thấy → coi như chưa trả lời
	}

	// Gom các ToolCallID đã có kết quả (các message role=tool SAU lastIdx).
	answered := make(map[string]bool, len(last.ToolCalls))
	for i := lastIdx + 1; i < len(s.Messages); i++ {
		if s.Messages[i].Role == provider.RoleTool && s.Messages[i].ToolCallID != "" {
			answered[s.Messages[i].ToolCallID] = true
		}
	}

	// Đếm tool calls chưa có kết quả.
	unanswered := 0
	for _, tc := range last.ToolCalls {
		if !answered[tc.ID] {
			unanswered++
		}
	}
	return unanswered
}
