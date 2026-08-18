package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// compactionMaxTokens là trần output cho 1 lượt tóm tắt — chỉ cần vài câu nên
// ngân sách nhỏ hơn nhiều so với reflectionMaxTokens (task nền, không nằm
// trên hot path).
const compactionMaxTokens = 800

// compactionTimeout giới hạn lượt gọi LLM tóm tắt. Khác reflection (chạy nền,
// không chặn response), lượt gọi này nằm NGAY TRÊN đường xử lý request — model
// chính phải chờ nó xong mới chạy tiếp — nên timeout phải ngắn hơn hẳn
// reflectionPerAttemptTimeout (40s) để không cộng thêm độ trễ lớn cho user.
const compactionTimeout = 12 * time.Second

// maxCompactionInputRunes giới hạn độ dài transcript đưa vào prompt tóm tắt,
// tính theo RUNE để không chẻ giữa ký tự tiếng Việt.
const maxCompactionInputRunes = 6000

const compactionSystemPrompt = `Bạn là hệ thống nén ngữ cảnh nội bộ cho AI Assistant. Nhiệm vụ DUY NHẤT: đọc đoạn tin nhắn cũ dưới đây và viết một đoạn tóm tắt NGẮN GỌN (3-6 câu), giữ lại thông tin QUAN TRỌNG mà các lượt trả lời SAU có thể cần: tên/thông tin người dùng đã cung cấp, quyết định đã chốt, kết quả tra cứu/tool cụ thể, số liệu/ID quan trọng. Bỏ qua chào hỏi xã giao và nội dung lặp lại. Trả lời THẲNG bằng đoạn tóm tắt, không thêm tiêu đề, không dùng markdown.`

// SummarizeMessages cố gắng tóm tắt THẬT msgs bằng 1 lượt gọi LLM (model rẻ/
// nhanh). Trả về (tóm tắt, true) khi thành công; ("", false) khi có BẤT KỲ lỗi
// nào (provider nil/rỗng, lỗi generate, lỗi giữa stream, hết thời gian, response
// rỗng) — đây là compaction BEST-EFFORT, KHÔNG retry (đường này vốn đã nhạy cảm
// độ trễ vì nằm trên hot path của request). Caller BẮT BUỘC phải fallback về
// placeholder TRUNG THỰC (không giả vờ đã tóm tắt) khi ok=false.
func SummarizeMessages(ctx context.Context, prov provider.Provider, model string, msgs []provider.Message) (summary string, ok bool) {
	if prov == nil || model == "" || len(msgs) == 0 {
		return "", false
	}

	var transcript strings.Builder
	for _, m := range msgs {
		content := m.Content
		if content == "" && len(m.ToolCalls) > 0 {
			names := make([]string, 0, len(m.ToolCalls))
			for _, tc := range m.ToolCalls {
				names = append(names, tc.Name)
			}
			content = fmt.Sprintf("[gọi tool: %s]", strings.Join(names, ", "))
		}
		if content == "" {
			continue
		}
		transcript.WriteString(fmt.Sprintf("%s: %s\n", strings.ToUpper(string(m.Role)), content))
	}

	trimmedConv := transcript.String()
	if runes := []rune(trimmedConv); len(runes) > maxCompactionInputRunes {
		// Giữ phần GẦN NHẤT (đuôi) — sát với nội dung sắp bị drop, nhiều khả
		// năng liên quan tới ngữ cảnh hiện tại hơn phần đầu quá xa.
		trimmedConv = string(runes[len(runes)-maxCompactionInputRunes:])
	}
	if strings.TrimSpace(trimmedConv) == "" {
		return "", false
	}

	callCtx, cancel := context.WithTimeout(ctx, compactionTimeout)
	defer cancel()

	req := provider.GenerateRequest{
		System: compactionSystemPrompt,
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: fmt.Sprintf("Tóm tắt đoạn hội thoại sau:\n\n%s", trimmedConv)},
		},
		Options: provider.ProviderOptions{
			Model:     model,
			MaxTokens: compactionMaxTokens,
			// Task nén ngữ cảnh không cần suy luận dài — model mặc định bật
			// thinking (vd deepseek-v4) có thể ăn vào ngân sách MaxTokens nhỏ
			// ở trên nếu không tắt (xem lý do tương tự ở reflection.go).
			ThinkingLevel: provider.ThinkingOff,
		},
	}

	stream, err := prov.Generate(callCtx, req)
	if err != nil {
		slog.Warn("agent: compaction generate lỗi — bỏ qua, dùng fallback trung thực", "err", err)
		return "", false
	}

	var out strings.Builder
	var streamErr error
	for chunk := range stream {
		switch chunk.Kind {
		case provider.ChunkText:
			out.WriteString(chunk.Text)
		case provider.ChunkError:
			streamErr = chunk.Err
		}
	}
	if streamErr != nil {
		slog.Warn("agent: compaction stream lỗi — bỏ qua, dùng fallback trung thực", "err", streamErr)
		return "", false
	}
	if ctxErr := callCtx.Err(); ctxErr != nil {
		slog.Warn("agent: compaction hết thời gian — bỏ qua, dùng fallback trung thực", "err", ctxErr)
		return "", false
	}

	summary = strings.TrimSpace(out.String())
	if summary == "" {
		return "", false
	}
	return summary, true
}

// SafeDropBoundary điều chỉnh dropCount để KHÔNG cắt giữa 1 cặp tool_call
// (assistant) / tool_result (role=tool): nếu tin nhắn giữ lại ĐẦU TIÊN là
// role=tool, nó là kết quả mồ côi của 1 tool_call vừa bị drop — provider (đặc
// biệt Anthropic, yêu cầu mọi tool_result phải khớp 1 tool_use đứng trước
// trong cùng request) sẽ từ chối request. Dịch dropCount tới khi gặp tin nhắn
// không phải role=tool, luôn giữ lại ít nhất 1 tin nhắn.
func SafeDropBoundary(msgs []provider.Message, dropCount int) int {
	if dropCount <= 0 {
		return 0
	}
	if dropCount >= len(msgs) {
		dropCount = len(msgs) - 1
	}
	for dropCount < len(msgs)-1 && msgs[dropCount].Role == provider.RoleTool {
		dropCount++
	}
	return dropCount
}
