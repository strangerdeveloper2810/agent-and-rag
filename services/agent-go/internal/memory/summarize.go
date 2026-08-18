package memory

import (
	"context"
	"fmt"

	"github.com/ai-agent-tut/agent-go/internal/agent"
	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// summarizeThreshold là ngường số message kích hoạt tóm tắt.
const summarizeThreshold = 15

// SummarizeNode trả về một agent.Node: nếu Messages > 15, cố gắng TÓM TẮT
// THẬT (qua 1 lượt LLM rẻ/nhanh, xem agent.SummarizeMessages) phần tin nhắn
// cũ vượt ngưỡng, rồi thay bằng 1 note chứa nội dung tóm tắt. Nếu lượt gọi
// LLM lỗi/timeout/rỗng, note sẽ nói THẬT là đã lược bỏ — KHÔNG giả vờ đã tóm
// tắt như bản cũ. Luôn trả về NodeModel.
//
// Role=user (không phải RoleSystem): adapter Anthropic BỎ QUA hoàn toàn mọi
// message role=system nằm trong Messages (system prompt đi qua field System
// riêng) — dùng RoleSystem ở đây sẽ khiến nội dung tóm tắt biến mất âm thầm
// mỗi khi Anthropic là provider đang phục vụ request.
//
// Cách dùng:
//
//	engine.SetMemoryNodes(..., memory.SummarizeNode(prov, fastModel))
func SummarizeNode(prov provider.Provider, model string) agent.Node {
	return func(ctx context.Context, s *agent.State, emit agent.EmitFunc) (agent.NodeID, error) {
		if len(s.Messages) <= summarizeThreshold {
			return agent.NodeModel, nil
		}

		// SafeDropBoundary tránh cắt giữa 1 cặp tool_call/tool_result. Luôn > 0
		// tại đây: input len(s.Messages)-summarizeThreshold > 0 (đã check ở
		// trên) và SafeDropBoundary chỉ TĂNG dropCount, không bao giờ giảm.
		dropCount := agent.SafeDropBoundary(s.Messages, len(s.Messages)-summarizeThreshold)
		dropped := s.Messages[:dropCount]

		var noteContent string
		if summary, ok := agent.SummarizeMessages(ctx, prov, model, dropped); ok {
			noteContent = fmt.Sprintf("[Tóm tắt %d tin nhắn trước]: %s", dropCount, summary)
		} else {
			noteContent = fmt.Sprintf("[%d tin nhắn trước đã bị lược bỏ do hội thoại quá dài — không tóm tắt được]", dropCount)
		}

		kept := make([]provider.Message, 0, len(s.Messages)-dropCount+1)
		kept = append(kept, provider.Message{
			Role:    provider.RoleUser,
			Content: noteContent,
		})
		kept = append(kept, s.Messages[dropCount:]...)
		s.Messages = kept

		emit(agent.MemoryEvent(fmt.Sprintf("summarized: condensed %d messages, keeping %d", dropCount, len(s.Messages)-1)))

		return agent.NodeModel, nil
	}
}
