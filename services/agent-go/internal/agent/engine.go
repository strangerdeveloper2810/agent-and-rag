package agent

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/guardrails"
	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/skills"
	"github.com/ai-agent-tut/agent-go/internal/tools"
)

// Engine là trái tim của agent runtime — chạy vòng lặp ReAct:
// recall → summarize → model → route → tools → model → route → ... → extract → end.
//
// Engine được inject Provider + Tool Registry qua constructor (DI).
// Memory nodes (recall, extract, summarize) được inject qua SetMemoryNodes
// để tránh import cycle giữa agent và memory package.
// Mọi I/O đều qua interface → test được với FakeProvider + Echo tool.
type Engine struct {
	prov     provider.Provider
	registry *tools.Registry

	// System prompt — injected before each LLM call
	systemPrompt string

	// Skill loader for progressive disclosure — skills are loaded on demand
	// when user input matches a skill's trigger keywords.
	skillLoader *skills.Loader

	// Memory node implementations — set via SetMemoryNodes.
	// nil = skip node (fallback: jump to next logical node).
	recallFn    Node
	extractFn   Node
	summarizeFn Node

	// Planning node implementations — set via SetPlanningNodes.
	// nil = skip node (fallback: jump to next logical node).
	planFn    Node
	reflectFn Node

	// Circuit breaker detects stuck loops (same tool+args called consecutively).
	// nil = disabled.
	circuitBreaker *guardrails.CircuitBreaker

	// Dynamic thinking: auto-adjust thinking level based on task complexity.
	dynamicThinking DynamicThinkingConfig

	// MaxContextTokens is the token budget before context trimming kicks in.
	// 0 = unlimited (no trimming). Default: 100000.
	maxContextTokens int
}

// SetDynamicThinking enables auto-adjusting thinking mode.
func (e *Engine) SetDynamicThinking(cfg DynamicThinkingConfig) {
	e.dynamicThinking = cfg
}

func (e *Engine) getDynamicThinking() DynamicThinkingConfig {
	return e.dynamicThinking
}

// NewEngine tạo Engine với provider và tool registry cho trước.
func NewEngine(prov provider.Provider, registry *tools.Registry) *Engine {
	return &Engine{
		prov:             prov,
		registry:         registry,
		maxContextTokens: 100000,
	}
}

// SetMemoryNodes gán các node memory (recall, extract, summarize).
// Dùng factory từ memory package: engine.SetMemoryNodes(memory.RecallNode(store), ...)
// nil node → node bị skip khi dispatch (fallback an toàn).
func (e *Engine) SetMemoryNodes(recall, extract, summarize Node) {
	e.recallFn = recall
	e.extractFn = extract
	e.summarizeFn = summarize
}

// SetPlanningNodes gán các node plan và reflect.
// nil node → node bị skip khi dispatch (fallback an toàn).
func (e *Engine) SetPlanningNodes(plan, reflect Node) {
	e.planFn = plan
	e.reflectFn = reflect
}

// getProvider / getRegistry / getSystemPrompt / getSkillLoader — implements modelEngine & toolsEngine.
func (e *Engine) getProvider() provider.Provider { return e.prov }
func (e *Engine) getRegistry() *tools.Registry   { return e.registry }
func (e *Engine) getSystemPrompt() string        { return e.systemPrompt }
func (e *Engine) getSkillLoader() *skills.Loader { return e.skillLoader }

// SetSystemPrompt sets the system prompt used for every LLM call.
func (e *Engine) SetSystemPrompt(prompt string) {
	e.systemPrompt = prompt
}

// SetSkillLoader sets the skills loader for progressive disclosure.
// When nil, skill matching is disabled.
func (e *Engine) SetSkillLoader(l *skills.Loader) {
	e.skillLoader = l
}

// SetCircuitBreaker sets the circuit breaker for stuck-loop detection.
// Pass nil to disable.
func (e *Engine) SetCircuitBreaker(cb *guardrails.CircuitBreaker) {
	e.circuitBreaker = cb
}

// SetMaxContextTokens sets the token budget before context trimming kicks in.
// 0 = unlimited (no trimming).
func (e *Engine) SetMaxContextTokens(n int) {
	e.maxContextTokens = n
}

// getMaxContextTokens implements modelEngine.
func (e *Engine) getMaxContextTokens() int { return e.maxContextTokens }

// Run chạy agent loop cho một lượt chat.
//
// Flow:
//
//	for {
//	    ctx.Err()? → return
//	    dispatch(node, state, emit) → nextNode
//	    nextNode == END → break
//	    node = nextNode
//	}
//	emit(DoneEvent)
//
// Run nhận ctx từ HTTP handler → khi client disconnect, ctx bị cancel
// → loop dừng ở lần check tiếp theo.
func (e *Engine) Run(ctx context.Context, in RunInput, emit EmitFunc) (provider.Usage, error) {
	start := time.Now()
	s := newState(in)
	node := NodeRecall

	slog.Info("engine: run started", "provider", e.prov.Name(), "maxSteps", s.MaxSteps)

	for {
		select {
		case <-ctx.Done():
			slog.Warn("engine: cancelled", "step", s.Step)
			return s.Usage, ctx.Err()
		default:
		}

		emit(StepEvent(node))
		stepStart := time.Now()

		next, err := e.dispatch(ctx, node, s, emit)
		elapsed := time.Since(stepStart)
		if err != nil {
			slog.Error("engine: dispatch failed", "node", node, "step", s.Step, "err", err)
			emit(ErrorEvent(err.Error()))
			return s.Usage, fmt.Errorf("engine: dispatch %s: %w", node, err)
		}

		slog.Info("engine: step done", "node", node, "next", next, "step", s.Step, "elapsed", elapsed.Round(time.Millisecond))

		if next == NodeEnd {
			break
		}
		node = next
	}

	slog.Info("engine: run done", "steps", s.Step, "total_ms", time.Since(start).Milliseconds(),
		"tokens_in", s.Usage.InputTokens, "tokens_out", s.Usage.OutputTokens, "total_tokens", s.TotalTokens)
	emit(DoneEvent(s.Usage, s.TotalTokens))
	return s.Usage, nil
}

// dispatch gọi đúng node function dựa trên NodeID.
// Thêm node mới (recall, plan, reflect...) chỉ cần thêm case.
func (e *Engine) dispatch(ctx context.Context, node NodeID, s *State, emit EmitFunc) (NodeID, error) {
	switch node {
	case NodeRecall:
		if e.recallFn != nil {
			return e.recallFn(ctx, s, emit)
		}
		// Fallback: không có recall → vào summarize → plan → model
		return NodeSummarize, nil
	case NodeSummarize:
		if e.summarizeFn != nil {
			return e.summarizeFn(ctx, s, emit)
		}
		return NodePlan, nil
	case NodePlan:
		if e.planFn != nil {
			return e.planFn(ctx, s, emit)
		}
		return NodeModel, nil
	case NodeModel:
		return nodeModel(ctx, e, s, emit)
	case NodeTools:
		// Circuit breaker: detect stuck loops (same tool+args called consecutively).
		if e.circuitBreaker != nil {
			last := s.LastAssistant()
			if last != nil {
				for _, tc := range last.ToolCalls {
					if err := e.circuitBreaker.Record(tc.Name, tc.Args); err != nil {
						emit(ErrorEvent(err.Error()))
						return NodeEnd, nil
					}
				}
			}
		}
		return nodeTools(ctx, e, s, emit)
	case NodeReflect:
		if e.reflectFn != nil {
			return e.reflectFn(ctx, s, emit)
		}
		return NodeExtract, nil
	case NodeExtract:
		if e.extractFn != nil {
			return e.extractFn(ctx, s, emit)
		}
		return NodeEnd, nil
	default:
		return NodeEnd, fmt.Errorf("engine: unknown node %q", node)
	}
}

// Provider returns the engine's LLM provider (for health checks, etc.)
func (e *Engine) Provider() provider.Provider { return e.prov }

// Registry returns the engine's tool registry (for health checks, etc.)
func (e *Engine) Registry() *tools.Registry { return e.registry }
