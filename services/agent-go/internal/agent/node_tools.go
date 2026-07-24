package agent

import (
	"context"

	"github.com/ai-agent-tut/agent-go/internal/tools"
)

// toolsEngine là interface engine cung cấp cho node tools.
type toolsEngine interface {
	getRegistry() *tools.Registry
}

// nodeTools chạy tất cả tool calls từ assistant message cuối cùng,
// song song qua registry.RunParallel, rồi append kết quả vào State.
//
// Flow:
//  1. Lấy LastAssistant → đọc ToolCalls
//  2. registry.RunParallel(ctx, toolCalls) — fan-out goroutine
//  3. Mỗi CallResult → Observation → AppendObservation
//  4. Emit ToolStartEvent / ToolEndEvent cho từng tool
//  5. return NodeModel (luôn quay lại model sau khi chạy tools)
func nodeTools(ctx context.Context, eng toolsEngine, s *State, emit EmitFunc) (NodeID, error) {
	last := s.LastAssistant()
	if last == nil || len(last.ToolCalls) == 0 {
		// Không có tool call nào → về model (edge case, router lẽ ra không gửi vào đây)
		return NodeModel, nil
	}

	reg := eng.getRegistry()

	// Emit tool_start cho từng tool
	for _, tc := range last.ToolCalls {
		emit(ToolStartEvent(tc.Name))
	}

	// Fan-out: chạy tất cả tool calls song song
	results := reg.RunParallel(ctx, last.ToolCalls)

	// Ghi kết quả vào state
	for i, res := range results {
		obs := Observation{
			CallID: res.Call.ID,
			Name:   res.Call.Name,
			Output: res.Result.Content,
		}
		if res.Err != nil {
			obs.Error = res.Err.Error()
			emit(ToolEndEvent(res.Call.Name, false, res.Err.Error()))
		} else {
			emit(ToolEndEvent(res.Call.Name, true, ""))
		}
		_ = i // i dùng để map với results
		s.AppendObservation(obs)
	}

	// Luôn quay lại model để LLM xem kết quả tool
	return NodeModel, nil
}

// compile-time check: nodeTools matches Node signature
var _ Node = func(ctx context.Context, s *State, emit EmitFunc) (NodeID, error) {
	// Placeholder: implementation needs engine reference
	return NodeModel, nil
}
