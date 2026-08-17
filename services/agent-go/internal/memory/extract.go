package memory

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/ai-agent-tut/agent-go/internal/agent"
	"github.com/ai-agent-tut/agent-go/internal/middleware"
	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// extractRule describes a pattern: regex + meaningful key for store.
type extractRule struct {
	re  *regexp.Regexp
	key string // meaningful key, e.g. "user_name", "like", "user_location"
}

// extractPatterns are patterns to extract memories from conversation.
// Keys use English names so recall can find them by semantic meaning.
var extractPatterns = []extractRule{
	{re: regexp.MustCompile(`(?i)tôi (?:tên|là) (?:tên |là )?(.+)$`), key: "user_name"},
	{re: regexp.MustCompile(`(?i)gọi tôi là (.+)`), key: "user_name"},
	{re: regexp.MustCompile(`(?i)tôi thích (.+)`), key: "like"},
	{re: regexp.MustCompile(`(?i)tôi (?:rất |cực |siêu )?thích (.+)`), key: "like"},
	{re: regexp.MustCompile(`(?i)tôi không thích (.+)`), key: "dislike"},
	{re: regexp.MustCompile(`(?i)tôi (?:ghét|không ưa) (.+)`), key: "dislike"},
	{re: regexp.MustCompile(`(?i)nhớ (?:là |rằng |giúp tôi |cho tôi )?(.+)$`), key: "fact"},
	{re: regexp.MustCompile(`(?i)hãy nhớ (.+)`), key: "fact"},
	{re: regexp.MustCompile(`(?i)tôi ở (.+)`), key: "user_location"},
	{re: regexp.MustCompile(`(?i)tôi (?:sống|đang) ở (.+)`), key: "user_location"},
	{re: regexp.MustCompile(`(?i)tôi làm (?:việc|ở|tại) (.+)`), key: "user_job"},
	{re: regexp.MustCompile(`(?i)tôi (?:là|đang là) (.+developer|.+engineer|.+designer|sinh viên|học sinh)`), key: "user_job"},
	{re: regexp.MustCompile(`(?i)tôi muốn (.+)`), key: "want"},
	{re: regexp.MustCompile(`(?i)tôi cần (.+)`), key: "need"},
	{re: regexp.MustCompile(`(?i)địa chỉ email (?:của tôi |là )?(.+)`), key: "email"},
	{re: regexp.MustCompile(`(?i)số điện thoại (?:của tôi |là )?(.+)`), key: "phone"},
}

// ExtractNode trả về một agent.Node: quét toàn bộ Messages để tìm pattern,
// lưu vào Store với key có ý nghĩa, emit memory event, rồi trả về NodeEnd.
func ExtractNode(store *Store) agent.Node {
	return func(ctx context.Context, s *agent.State, emit agent.EmitFunc) (agent.NodeID, error) {
		tenantID := middleware.GetTenantID(ctx)

		extracted := 0
		seen := make(map[string]bool)

		for _, msg := range s.Messages {
			if msg.Role != provider.RoleUser && msg.Role != provider.RoleAssistant {
				continue
			}
			for _, rule := range extractPatterns {
				matches := rule.re.FindStringSubmatch(msg.Content)
				if len(matches) < 2 {
					continue
				}
				value := strings.TrimSpace(matches[1])
				if value == "" || len(value) > 200 {
					continue
				}
				if seen[rule.key] {
					continue
				}
				seen[rule.key] = true
				store.Set(tenantID, rule.key, value)
				extracted++
				emit(agent.MemoryEvent(fmt.Sprintf("extracted: %s = %s", rule.key, value)))
			}
		}

		return agent.NodeEnd, nil
	}
}
