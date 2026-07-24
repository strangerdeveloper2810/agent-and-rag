package agent

import (
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

func TestRoute_ToolCalls(t *testing.T) {
	// Assistant cuối có tool_calls → phải chạy tools.
	s := &State{
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "tạo task mua sữa"},
			{Role: provider.RoleAssistant, Content: "", ToolCalls: []provider.ToolCall{
				{ID: "c1", Name: "createTask", Args: nil},
			}},
		},
	}
	if got := route(s); got != NodeTools {
		t.Fatalf("route() = %q, want %q", got, NodeTools)
	}
}

func TestRoute_MultipleToolCalls(t *testing.T) {
	// Nhiều tool_call trong cùng 1 assistant message → vẫn về NodeTools.
	s := &State{
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "tìm tài liệu rồi tạo task"},
			{Role: provider.RoleAssistant, Content: "Để tôi tra cứu...", ToolCalls: []provider.ToolCall{
				{ID: "c1", Name: "ragSearch", Args: nil},
				{ID: "c2", Name: "createTask", Args: nil},
			}},
		},
	}
	if got := route(s); got != NodeTools {
		t.Fatalf("route() = %q, want %q", got, NodeTools)
	}
}

func TestRoute_FinalAnswer(t *testing.T) {
	// Assistant cuối trả lời text (không tool_call) → extract.
	s := &State{
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "cảm ơn"},
			{Role: provider.RoleAssistant, Content: "Không có gì! Cần gì nữa không?"},
		},
	}
	if got := route(s); got != NodeExtract {
		t.Fatalf("route() = %q, want %q", got, NodeExtract)
	}
}

func TestRoute_MaxStepsExceeded(t *testing.T) {
	// Vượt maxSteps → dừng, kể cả đang có tool_call.
	s := &State{
		Step:     12,
		MaxSteps: 12,
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "tạo task"},
			{Role: provider.RoleAssistant, Content: "", ToolCalls: []provider.ToolCall{
				{ID: "c1", Name: "createTask", Args: nil},
			}},
		},
	}
	if got := route(s); got != NodeEnd {
		t.Fatalf("route() = %q, want %q (maxSteps exceeded)", got, NodeEnd)
	}
}

func TestRoute_Interrupt(t *testing.T) {
	// Có Interrupt → ưu tiên cao nhất, về NodeInterrupt.
	s := &State{
		Interrupt: &Interrupt{Reason: "confirm_destructive", Tool: "deleteTask"},
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "xóa task 123"},
			{Role: provider.RoleAssistant, Content: "", ToolCalls: []provider.ToolCall{
				{ID: "c1", Name: "deleteTask", Args: nil},
			}},
		},
	}
	if got := route(s); got != NodeInterrupt {
		t.Fatalf("route() = %q, want %q", got, NodeInterrupt)
	}
}

func TestRoute_NoMessages(t *testing.T) {
	// Chưa có assistant message nào → extract (chưa có gì để route).
	s := &State{
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "hello"},
		},
	}
	if got := route(s); got != NodeExtract {
		t.Fatalf("route() = %q, want %q", got, NodeExtract)
	}
}

func TestRoute_AllToolResultsProcessed(t *testing.T) {
	// Assistant đã gọi tool, tool đã chạy xong (có tool result trong messages),
	// nhưng assistant CHƯA phản hồi lại → router vẫn cần chạy model tiếp,
	// không phải tools (vì tool calls đã được xử lý).
	// Kịch bản: model → route thấy lần cuối assistant có tool_calls → tools.
	// Sau tools, assistant message CUỐI vẫn là message có tool_calls.
	// Router chỉ nhìn assistant cuối → vẫn thấy tool_calls → tools AGAIN → loop vô hạn!
	//
	// ĐÂY LÀ BUG TIỀM ẨN: sau khi tools chạy, tool results được append,
	// nhưng assistant message gốc (có tool_calls) vẫn là LastAssistant().
	// Router cần kiểm tra xem tool calls ĐÃ được trả lời chưa.
	//
	// Fix: router kiểm tra: có assistant với tool_calls, NHƯNG tool calls
	// đó đã có tool result tương ứng chưa? Nếu có rồi → không vào tools nữa → model.
}

func TestRoute_ToolCallsAlreadyAnswered(t *testing.T) {
	// Assistant gọi tool → tools đã chạy → có tool result message.
	// Sau khi tools done → luôn quay lại model để LLM phản hồi.
	s := &State{
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "tạo task mua sữa"},
			{Role: provider.RoleAssistant, Content: "", ToolCalls: []provider.ToolCall{
				{ID: "c1", Name: "createTask", Args: nil},
			}},
			// Tool result đã có → tất cả tool calls đã được trả lời.
			{Role: provider.RoleTool, ToolCallID: "c1", Content: `{"ok":true}`},
		},
	}
	if got := route(s); got != NodeModel {
		t.Fatalf("route() = %q, want %q (all tool calls answered → back to model)", got, NodeModel)
	}
}

func TestRoute_ToolCallsPartiallyAnswered(t *testing.T) {
	// Assistant gọi 2 tool, mới có kết quả của 1 → vẫn cần chờ tools.
	// Nhưng thực tế: tools chạy fan-out song song, tất cả xong cùng lúc.
	// Trường hợp này edge case: không nên xảy ra trong engine hiện tại,
	// nhưng router nên xử lý đúng.
	s := &State{
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "tìm và tạo task"},
			{Role: provider.RoleAssistant, Content: "", ToolCalls: []provider.ToolCall{
				{ID: "c1", Name: "ragSearch", Args: nil},
				{ID: "c2", Name: "createTask", Args: nil},
			}},
			// Mới có kết quả của c1, c2 chưa có.
			{Role: provider.RoleTool, ToolCallID: "c1", Content: `[{"doc":"x"}]`},
		},
	}
	if got := route(s); got != NodeTools {
		t.Fatalf("route() = %q, want %q (not all tool calls answered → need tools)", got, NodeTools)
	}
}
