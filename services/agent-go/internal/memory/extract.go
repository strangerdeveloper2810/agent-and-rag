package memory

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/ai-agent-tut/agent-go/internal/agent"
	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// extractRule mô tả một pattern trích xuất: regex + prefix cho key.
type extractRule struct {
	re      *regexp.Regexp
	keyPrefix string // prefix cho key lưu trong store, vd "pref:", "fact:"
}

// extractPatterns là danh sách pattern trích xuất memory từ hội thoại.
var extractPatterns = []extractRule{
	{re: regexp.MustCompile(`(?i)tôi thích (.+)`), keyPrefix: "pref"},
	{re: regexp.MustCompile(`(?i)tôi không thích (.+)`), keyPrefix: "pref"},
	{re: regexp.MustCompile(`(?i)nhớ là (.+)`), keyPrefix: "fact"},
	{re: regexp.MustCompile(`(?i)hãy nhớ (.+)`), keyPrefix: "fact"},
	{re: regexp.MustCompile(`(?i)tôi tên là (.+)`), keyPrefix: "fact"},
	{re: regexp.MustCompile(`(?i)tôi ở (.+)`), keyPrefix: "fact"},
	{re: regexp.MustCompile(`(?i)tôi làm (.+)`), keyPrefix: "fact"},
	{re: regexp.MustCompile(`(?i)tôi muốn (.+)`), keyPrefix: "pref"},
}

// ExtractNode trả về một agent.Node: quét toàn bộ Messages để tìm pattern,
// lưu vào Store, emit memory event, rồi trả về NodeEnd.
//
// Cách dùng:
//
//	store := memory.NewStore()
//	engine.SetMemoryNodes(..., memory.ExtractNode(store), ...)
func ExtractNode(store *Store) agent.Node {
	return func(ctx context.Context, s *agent.State, emit agent.EmitFunc) (agent.NodeID, error) {
		_ = ctx

		extracted := 0
		seen := make(map[string]bool) // tránh trùng trong cùng 1 lượt

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
				if value == "" {
					continue
				}
				key := rule.keyPrefix + ":" + value[:min(40, len(value))]
				if seen[key] {
					continue
				}
				seen[key] = true
				store.Set(key, value)
				extracted++
				emit(agent.MemoryEvent(fmt.Sprintf("extracted: %s = %s", key, value)))
			}
		}

		return agent.NodeEnd, nil
	}
}
