package agent

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/guardrails"
	"github.com/ai-agent-tut/agent-go/internal/provider"
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

	var safeCalls []provider.ToolCall
	var destructiveCalls []provider.ToolCall

	for _, tc := range last.ToolCalls {
		t, ok := reg.Get(tc.Name)
		if !ok {
			safeCalls = append(safeCalls, tc)
			continue
		}
		if err := guardrails.CheckTool(t); err != nil {
			var needConf *guardrails.NeedConfirmationError
			if errors.As(err, &needConf) {
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
	}

	for _, tc := range safeCalls {
		emit(ToolStartEvent(tc.Name))
	}

	if len(safeCalls) > 0 {
		results := reg.RunParallel(ctx, safeCalls)

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
	}

	if s.Interrupt != nil {
		return NodeInterrupt, nil
	}
	return NodeModel, nil
}

// compile-time check
var _ Node = func(ctx context.Context, s *State, emit EmitFunc) (NodeID, error) {
	return NodeModel, nil
}
