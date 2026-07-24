package agent

import (
	"context"
	"fmt"

	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/tools"
)

// Engine là trái tim của agent runtime — chạy vòng lặp ReAct:
// model → route → tools → model → route → ... → end.
//
// Engine được inject Provider + Tool Registry qua constructor (DI).
// Mọi I/O đều qua interface → test được với FakeProvider + Echo tool.
type Engine struct {
	prov     provider.Provider
	registry *tools.Registry
}

// NewEngine tạo Engine với provider và tool registry cho trước.
func NewEngine(prov provider.Provider, registry *tools.Registry) *Engine {
	return &Engine{prov: prov, registry: registry}
}

// getProvider / getRegistry — implements modelEngine & toolsEngine interfaces.
func (e *Engine) getProvider() provider.Provider { return e.prov }
func (e *Engine) getRegistry() *tools.Registry   { return e.registry }

// Run chạy agent loop cho một lượt chat.
//
// Flow:
//   for {
//       ctx.Err()? → return
//       dispatch(node, state, emit) → nextNode
//       nextNode == END → break
//       node = nextNode
//   }
//   emit(DoneEvent)
//
// Run nhận ctx từ HTTP handler → khi client disconnect, ctx bị cancel
// → loop dừng ở lần check tiếp theo.
func (e *Engine) Run(ctx context.Context, in RunInput, emit EmitFunc) (provider.Usage, error) {
	s := newState(in)
	node := NodeModel

	for {
		// Kiểm tra cancellation MỖI vòng lặp
		select {
		case <-ctx.Done():
			return s.Usage, ctx.Err()
		default:
		}

		emit(StepEvent(node))

		next, err := e.dispatch(ctx, node, s, emit)
		if err != nil {
			emit(ErrorEvent(err.Error()))
			return s.Usage, fmt.Errorf("engine: dispatch %s: %w", node, err)
		}

		if next == NodeEnd {
			break
		}
		node = next
	}

	emit(DoneEvent(s.Usage))
	return s.Usage, nil
}

// dispatch gọi đúng node function dựa trên NodeID.
// Thêm node mới (recall, plan, reflect...) chỉ cần thêm case.
func (e *Engine) dispatch(ctx context.Context, node NodeID, s *State, emit EmitFunc) (NodeID, error) {
	switch node {
	case NodeModel:
		return nodeModel(ctx, e, s, emit)
	case NodeTools:
		return nodeTools(ctx, e, s, emit)
	default:
		return NodeEnd, fmt.Errorf("engine: unknown node %q", node)
	}
}

// Provider returns the engine's LLM provider (for health checks, etc.)
func (e *Engine) Provider() provider.Provider { return e.prov }

// Registry returns the engine's tool registry (for health checks, etc.)
func (e *Engine) Registry() *tools.Registry { return e.registry }
