package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/guardrails"
	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/tools"
)

type toolsEngine interface {
	getRegistry() *tools.Registry
	getMaxToolOutput() int
	getAllowDestructiveTools() bool
}

// defaultMaxToolOutput khớp cfg.MaxToolOutput (24000) — giá trị mặc định khi
// caller không gọi Engine.SetMaxToolOutput.
const defaultMaxToolOutput = 24000

// nodeTools chạy tất cả tool calls từ assistant message cuối cùng, song song.
func nodeTools(ctx context.Context, eng toolsEngine, s *State, emit EmitFunc) (NodeID, error) {
	last := s.LastAssistant()
	if last == nil || len(last.ToolCalls) == 0 {
		return NodeModel, nil
	}

	toolNames := make([]string, len(last.ToolCalls))
	for i, tc := range last.ToolCalls {
		toolNames[i] = tc.Name
	}
	slog.Info("tools: executing", "count", len(last.ToolCalls), "tools", toolNames)

	start := time.Now()
	reg := eng.getRegistry()

	var safeCalls []provider.ToolCall
	var destructiveCalls []provider.ToolCall
	allowDestructive := eng.getAllowDestructiveTools()

	for _, tc := range last.ToolCalls {
		t, ok := reg.Get(tc.Name)
		if !ok {
			safeCalls = append(safeCalls, tc)
			continue
		}
		if err := guardrails.CheckTool(t); err != nil {
			var needConf *guardrails.NeedConfirmationError
			if errors.As(err, &needConf) {
				if allowDestructive {
					// Người dùng đã chủ động bật ALLOW_DESTRUCTIVE_TOOLS.
					slog.Warn("guardrails: cho phép tool destructive vì ALLOW_DESTRUCTIVE_TOOLS=true", "tool", tc.Name)
					safeCalls = append(safeCalls, tc)
					continue
				}
				destructiveCalls = append(destructiveCalls, tc)
				continue
			}
			slog.Warn("guardrails: unknown tool kind", "tool", tc.Name, "err", err)
		}
		safeCalls = append(safeCalls, tc)
	}

	if len(destructiveCalls) > 0 {
		for i, dc := range destructiveCalls {
			emit(InterruptEvent("confirm_destructive", dc.Name))
			if i == 0 {
				s.Interrupt = &Interrupt{
					Reason: "confirm_destructive",
					Tool:   dc.Name,
					Args:   string(dc.Args),
				}
			}
		}

		// Phát THÊM một đoạn text giải thích. Nếu không có bước này, engine
		// dừng ở NodeInterrupt và vẫn emit done bình thường, nhưng KHÔNG có
		// text nào — user nhận một bubble RỖNG HOÀN TOÀN, không lỗi, không lý
		// do, không cách nào tiếp tục. Emit từ tầng Go để MỌI client (web,
		// cmd/jarvis) đều thấy, thay vì bắt từng client tự dựng thông báo từ
		// event interrupt (web hiện đang bỏ qua event này).
		emit(TextEvent(destructiveBlockedMessage(destructiveCalls)))
	}

	for _, tc := range safeCalls {
		emit(ToolStartEvent(tc.Name))
	}

	if len(safeCalls) > 0 {
		// Stream kết quả: emit tool_end NGAY KHI từng tool hoàn thành (không
		// chờ cả nhóm) để UI hiện tiến độ trực tiếp trong lúc chờ.
		results := reg.RunParallelStreaming(ctx, safeCalls, func(i int, res tools.CallResult) {
			if res.Err != nil {
				slog.Error("tools: failed", "tool", res.Call.Name, "err", res.Err)
				emit(ToolEndEvent(res.Call.Name, false, res.Err.Error()))
			} else {
				emit(ToolEndEvent(res.Call.Name, true, toolResultPreview(res.Result.Content)))
			}
		})

		maxOut := eng.getMaxToolOutput()
		for _, res := range results {
			output, truncated := capToolOutput(res.Result.Content, maxOut)
			if truncated {
				slog.Warn("tools: output bị cắt để giữ context trong ngân sách",
					"tool", res.Call.Name,
					"original_chars", len([]rune(res.Result.Content)),
					"max_chars", maxOut)
			}
			obs := Observation{
				CallID: res.Call.ID,
				Name:   res.Call.Name,
				Output: output,
			}
			if res.Err != nil {
				obs.Error = res.Err.Error()
			} else {
				slog.Info("tools: done", "tool", res.Call.Name, "output_preview", truncateRunes(output, 100))
			}
			s.AppendObservation(obs)
		}

		slog.Info("tools: all done", "count", len(results), "elapsed_ms", time.Since(start).Milliseconds())
	}

	if s.Interrupt != nil {
		return NodeInterrupt, nil
	}
	return NodeModel, nil
}

// destructiveArgsPreviewMax giới hạn độ dài args hiện trong thông báo chặn.
const destructiveArgsPreviewMax = 200

// destructiveBlockedMessage dựng thông báo tiếng Việt giải thích tool nào bị
// chặn và cách xử lý, để user không nhận một câu trả lời rỗng không lời giải thích.
func destructiveBlockedMessage(calls []provider.ToolCall) string {
	var b strings.Builder
	b.WriteString("⚠ **Đã dừng: cần xác nhận trước khi chạy lệnh có thể thay đổi hệ thống**\n\n")
	if len(calls) == 1 {
		b.WriteString("Tôi cần chạy công cụ sau nhưng nó bị guardrails chặn lại:\n\n")
	} else {
		b.WriteString(fmt.Sprintf("Tôi cần chạy %d công cụ sau nhưng chúng bị guardrails chặn lại:\n\n", len(calls)))
	}
	for _, c := range calls {
		b.WriteString(fmt.Sprintf("- `%s`", c.Name))
		if args := strings.TrimSpace(string(c.Args)); args != "" && args != "{}" {
			b.WriteString(" với tham số: `" + truncateRunes(args, destructiveArgsPreviewMax) + "`")
		}
		b.WriteString("\n")
	}
	b.WriteString("\nĐây là công cụ được xếp loại `destructive` (có thể sửa/xoá dữ liệu trên máy), " +
		"nên mặc định hệ thống KHÔNG tự chạy.\n\n" +
		"Cách xử lý:\n" +
		"- Tự chạy lệnh đó trong terminal của sir rồi dán kết quả lại cho tôi, hoặc\n" +
		"- Bật `ALLOW_DESTRUCTIVE_TOOLS=true` trong file `.env` của agent rồi khởi động lại " +
		"nếu sir muốn tôi được phép tự chạy (chỉ nên bật trên máy cá nhân), hoặc\n" +
		"- Nhờ tôi làm cách khác không cần công cụ này.")
	return b.String()
}

// toolResultPreviewMax giới hạn độ dài preview output tool gửi kèm tool_end.
const toolResultPreviewMax = 300

// toolResultPreview trích đoạn ngắn từ output tool để stream cho UI.
func toolResultPreview(output string) string {
	return truncateRunes(strings.TrimSpace(output), toolResultPreviewMax)
}

// truncateRunes cắt chuỗi theo RUNE (không theo byte) và thêm dấu "…" khi cắt.
// Cắt theo byte trên text tiếng Việt sẽ chẻ giữa ký tự multi-byte, tạo ký tự
// lỗi trong log và trong preview hiện trên UI.
func truncateRunes(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "…"
}

// capToolOutput giới hạn output tool trước khi đưa vào context LLM. Trả thêm
// cờ cho biết có bị cắt hay không để caller log.
//
// Khi cắt, phần đuôi được thay bằng ghi chú tường minh để LLM BIẾT là dữ liệu
// chưa đầy đủ — nếu cắt âm thầm, model sẽ kết luận trên dữ liệu thiếu mà tưởng
// là đủ (vd đọc 1 phần danh sách file rồi khẳng định "chỉ có N file").
func capToolOutput(output string, maxRunes int) (string, bool) {
	if maxRunes <= 0 {
		return output, false
	}
	runes := []rune(output)
	if len(runes) <= maxRunes {
		return output, false
	}
	return string(runes[:maxRunes]) +
		fmt.Sprintf("\n\n[... output bị cắt: %d ký tự đầu trong tổng %d. Dữ liệu KHÔNG đầy đủ — nếu cần phần còn lại, hãy gọi lại tool với truy vấn/bộ lọc hẹp hơn.]",
			maxRunes, len(runes)), true
}

// compile-time check
var _ Node = func(ctx context.Context, s *State, emit EmitFunc) (NodeID, error) {
	return NodeModel, nil
}
