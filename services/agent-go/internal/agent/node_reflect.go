package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// nodeReflect đánh giá tiến độ plan sau khi model sinh phản hồi.
// Nếu plan còn bước chưa hoàn thành, route về model để tiếp tục.
// Nếu không có plan hoặc plan đã xong, route về extract.
// Chạy giữa model và extract.
func nodeReflect(ctx context.Context, s *State, emit EmitFunc) (NodeID, error) {
	if len(s.Plan) == 0 || s.PlanStep >= len(s.Plan) {
		return NodeExtract, nil
	}

	s.PlanStep++
	emit(ReflectEvent(s.PlanStep, len(s.Plan)))

	if s.PlanStep >= len(s.Plan) {
		slog.Info("reflect: plan complete", "steps", len(s.Plan))
		return NodeExtract, nil
	}

	nextStep := s.Plan[s.PlanStep]
	slog.Info("reflect: continuing plan", "step", s.PlanStep+1, "of", len(s.Plan), "next", nextStep)

	s.Messages = append(s.Messages, provider.Message{
		Role:    provider.RoleUser,
		Content: fmt.Sprintf("[Continue with plan step %d/%d: %s]", s.PlanStep+1, len(s.Plan), nextStep),
	})

	return NodeModel, nil
}
