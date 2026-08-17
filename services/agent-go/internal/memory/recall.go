package memory

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ai-agent-tut/agent-go/internal/agent"
	"github.com/ai-agent-tut/agent-go/internal/middleware"
	"github.com/ai-agent-tut/agent-go/internal/provider"
)

var keywordToKeys = map[string][]string{
	"tên":           {"user_name"},
	"name":          {"user_name"},
	"thích":         {"like"},
	"like":          {"like"},
	"ghét":          {"dislike"},
	"dislike":       {"dislike"},
	"ở":             {"user_location"},
	"sống":          {"user_location"},
	"location":      {"user_location"},
	"địa chỉ":       {"user_location"},
	"làm":           {"user_job"},
	"job":           {"user_job"},
	"nghề":          {"user_job"},
	"work":          {"user_job"},
	"email":         {"email"},
	"mail":          {"email"},
	"số điện thoại": {"phone"},
	"phone":         {"phone"},
	"sdt":           {"phone"},
	"muốn":          {"want"},
	"cần":           {"need"},
	"nhớ":           {"fact"},
	"remember":      {"fact"},
}

func RecallNode(store *Store) agent.Node {
	return func(ctx context.Context, s *agent.State, emit agent.EmitFunc) (agent.NodeID, error) {
		tenantID := middleware.GetTenantID(ctx)

		query := lastUserContent(s)
		if query == "" {
			return agent.NodeModel, nil
		}

		results := make(map[string]string)

		// Step 1: Direct key lookup from keyword mapping (fast, accurate).
		lower := strings.ToLower(query)
		for keyword, keys := range keywordToKeys {
			if strings.Contains(lower, keyword) {
				for _, k := range keys {
					if v, ok := store.Get(tenantID, k); ok {
						results[k] = v
					}
				}
			}
		}

		// Step 2: Full-text substring search as fallback.
		fullResults := store.Search(tenantID, query)
		for k, v := range fullResults {
			results[k] = v
		}

		// Step 3: Embedding-based semantic search (optional).
		semResults, err := store.SemanticSearch(tenantID, query, 5)
		if err != nil {
			slog.Warn("memory: semantic search failed, continuing with keyword results", "err", err)
		}
		for _, item := range semResults {
			if _, exists := results[item.Key]; !exists {
				results[item.Key] = item.Value
			}
		}

		if len(results) == 0 {
			return agent.NodeModel, nil
		}

		slog.Info("memory: recalled", "count", len(results), "tenant", tenantID)
		items := make([]string, 0, len(results))
		for k, v := range results {
			items = append(items, fmt.Sprintf("%s: %s", k, v))
		}

		// Feed recalled memories into State so nodeModel can weave them into
		// the system prompt for THIS request (previously only emitted for the
		// SSE UI stream and never reached the LLM — see node_model.go).
		s.RecalledMemories = items

		emit(agent.MemoryEvent("recalled: " + strings.Join(items, " | ")))

		return agent.NodeModel, nil
	}
}

func lastUserContent(s *agent.State) string {
	for i := len(s.Messages) - 1; i >= 0; i-- {
		if s.Messages[i].Role == provider.RoleUser {
			return s.Messages[i].Content
		}
	}
	return ""
}
