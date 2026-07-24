package memory

import (
	"context"
	"fmt"
	"strings"

	"github.com/ai-agent-tut/agent-go/internal/agent"
	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// RecallNode trả về một agent.Node: tìm trong Store các mục liên quan đến
// tin nhắn cuối cùng của user, emit memory event, rồi trả về NodeModel.
//
// Cách dùng (trong main.go):
//
//	store := memory.NewStore()
//	engine.SetMemoryNodes(memory.RecallNode(store), ...)
func RecallNode(store *Store) agent.Node {
	return func(ctx context.Context, s *agent.State, emit agent.EmitFunc) (agent.NodeID, error) {
		_ = ctx // tôn trọng signature; có thể dùng ctx timeout sau này

		// Tìm user message cuối cùng.
		query := lastUserContent(s)
		if query == "" {
			return agent.NodeModel, nil
		}

		results := store.Search(query)
		if len(results) == 0 {
			return agent.NodeModel, nil
		}

		// Gom kết quả cho event.
		items := make([]string, 0, len(results))
		for k, v := range results {
			items = append(items, fmt.Sprintf("%s: %s", k, v))
		}
		emit(agent.MemoryEvent("recalled: " + strings.Join(items, " | ")))

		return agent.NodeModel, nil
	}
}

// lastUserContent trả về Content của user message cuối cùng.
func lastUserContent(s *agent.State) string {
	for i := len(s.Messages) - 1; i >= 0; i-- {
		if s.Messages[i].Role == provider.RoleUser {
			return s.Messages[i].Content
		}
	}
	return ""
}
