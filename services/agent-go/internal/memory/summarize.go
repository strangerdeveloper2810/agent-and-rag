package memory

import (
	"context"
	"fmt"

	"github.com/ai-agent-tut/agent-go/internal/agent"
	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// summarizeThreshold là ngường số message kích hoạt tóm tắt.
const summarizeThreshold = 15

// SummarizeNode trả về một agent.Node: nếu Messages > 15, giữ lại 15 tin
// cuối + prepend 1 system note báo đã rút gọn. Luôn trả về NodeModel.
//
// Cách dùng:
//
//	engine.SetMemoryNodes(..., memory.SummarizeNode())
func SummarizeNode() agent.Node {
	return func(ctx context.Context, s *agent.State, emit agent.EmitFunc) (agent.NodeID, error) {
		_ = ctx

		if len(s.Messages) <= summarizeThreshold {
			return agent.NodeModel, nil
		}

		excess := len(s.Messages) - summarizeThreshold

		// Giữ last 15 và prepend summary note.
		kept := make([]provider.Message, 0, summarizeThreshold+1)
		kept = append(kept, provider.Message{
			Role:    provider.RoleSystem,
			Content: fmt.Sprintf("[Tóm tắt] %d tin nhắn trước đã được rút gọn.", excess),
		})
		kept = append(kept, s.Messages[excess:]...)
		s.Messages = kept

		emit(agent.MemoryEvent(fmt.Sprintf("summarized: condensed %d messages, keeping %d", excess, len(s.Messages)-1)))

		return agent.NodeModel, nil
	}
}
