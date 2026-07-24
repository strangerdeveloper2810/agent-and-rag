package agent

import (
	"context"
	"log/slog"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/tools"
)

type toolsEngine interface {
	getRegistry() *tools.Registry
}

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

	for _, tc := range last.ToolCalls {
		emit(ToolStartEvent(tc.Name))
	}

	results := reg.RunParallel(ctx, last.ToolCalls)

	for _, res := range results {
		obs := Observation{
			CallID: res.Call.ID,
			Name:   res.Call.Name,
			Output: res.Result.Content,
		}
		if res.Err != nil {
			obs.Error = res.Err.Error()
			slog.Error("tools: failed", "tool", res.Call.Name, "err", res.Err)
			emit(ToolEndEvent(res.Call.Name, false, res.Err.Error()))
		} else {
			outLen := len(res.Result.Content)
			if outLen > 100 {
				outLen = 100
			}
			slog.Info("tools: done", "tool", res.Call.Name, "output_preview", res.Result.Content[:outLen])
			emit(ToolEndEvent(res.Call.Name, true, ""))
		}
		s.AppendObservation(obs)
	}

	slog.Info("tools: all done", "count", len(results), "elapsed_ms", time.Since(start).Milliseconds())
	return NodeModel, nil
}

// compile-time check
var _ Node = func(ctx context.Context, s *State, emit EmitFunc) (NodeID, error) {
	return NodeModel, nil
}
