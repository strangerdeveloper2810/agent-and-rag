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

	// maxToolOutput giới hạn số KÝ TỰ output của mỗi tool được đưa vào
	// s.Messages. 0 = không giới hạn.
	//
	// Trước đây cfg.MaxToolOutput là CONFIG CHẾT (khai báo nhưng không nơi nào
	// đọc), và việc cắt output nằm rời rạc trong từng tool với các ngưỡng khác
	// nhau (8000 shell/git, 10000 http/json, 15000 web, 24000 files/rag) —
	// trong khi file.search và rag.search KHÔNG cắt gì cả. Một
	// file.search {"pattern":"*"} có thể đẩy hàng MB vào context, làm lượt LLM
	// sau đắt đột biến, lỗi provider, hoặc bị trimContext cắt mất ngữ cảnh cũ.
	// Đây là chốt an toàn TẬP TRUNG, áp cho mọi tool bất kể tool có tự cắt hay không.
	maxToolOutput int

	// allowDestructiveTools cho phép chạy tool KindDestructive không cần xác
	// nhận. false (mặc định) → guardrails chặn và agent giải thích cho user.
	allowDestructiveTools bool

	// maxOutputTokens là trần output token cho MỖI lần gọi LLM. 0 = không giới
	// hạn. Trước đây cfg.MaxTokens là config chết nên request luôn gửi
	// max_tokens=0 và không có trần nào — xem cfg.MaxTokens.
	maxOutputTokens int

	// ownerTenantIDs: tenant được dùng nhóm tool đặc quyền. Rỗng = chỉ tenant
	// "default" (local, không auth) — xem tools.IsOwnerTenant.
	ownerTenantIDs []string
}

// SetMaxOutputTokens đặt trần output token cho mỗi lần gọi LLM. n <= 0 = không giới hạn.
func (e *Engine) SetMaxOutputTokens(n int) {
	e.maxOutputTokens = n
}

func (e *Engine) getMaxOutputTokens() int { return e.maxOutputTokens }

// SetOwnerTenants khai báo các tenant được dùng nhóm tool đặc quyền (file.*,
// shell.exec, git) — xem tools.IsOwnerTenant và cfg.OwnerTenantIDs.
func (e *Engine) SetOwnerTenants(ids []string) {
	e.ownerTenantIDs = ids
}

func (e *Engine) getOwnerTenants() []string { return e.ownerTenantIDs }

// SetMaxToolOutput đặt giới hạn ký tự output mỗi tool đưa vào context.
// n <= 0 → không giới hạn.
func (e *Engine) SetMaxToolOutput(n int) {
	e.maxToolOutput = n
}

func (e *Engine) getMaxToolOutput() int { return e.maxToolOutput }

// SetAllowDestructiveTools cho phép agent tự chạy tool KindDestructive (shell.exec)
// mà không cần xác nhận. MẶC ĐỊNH false — xem cfg.AllowDestructiveTools.
func (e *Engine) SetAllowDestructiveTools(allow bool) {
	e.allowDestructiveTools = allow
}

func (e *Engine) getAllowDestructiveTools() bool { return e.allowDestructiveTools }

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
		maxToolOutput:    defaultMaxToolOutput,
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

// EnablePlanning bật node plan/reflect nội bộ: request phức tạp tốn thêm
// 1 LLM call (plan) trước token đầu tiên. TẮT mặc định.
func (e *Engine) EnablePlanning() {
	e.planFn = func(ctx context.Context, s *State, emit EmitFunc) (NodeID, error) {
		return nodePlan(ctx, e, s, emit)
	}
	e.reflectFn = nodeReflect
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

	// Breaker RIÊNG cho lượt chạy này. Trước đây engine dùng thẳng
	// e.circuitBreaker — MỘT instance chia sẻ cho cả 3 agent và toàn bộ
	// process, và Reset() không được gọi ở đâu trong production. Hệ quả: 2
	// request khác nhau (khác user) gọi cùng tool + cùng args thì request thứ 3
	// bị chặn "stuck loop" oan, tool không chạy và câu trả lời rỗng; ngược lại
	// 2 run song song ghi đè state của nhau nên loop thật lại không bị phát hiện.
	if e.circuitBreaker != nil {
		s.loopBreaker = guardrails.NewCircuitBreaker(e.circuitBreaker.MaxRepeats())
	}

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

		if next == NodeEnd || next == NodeInterrupt {
			break
		}
		node = next
	}

	slog.Info("engine: run done", "steps", s.Step, "total_ms", time.Since(start).Milliseconds(),
		"tokens_in", s.Usage.InputTokens, "tokens_out", s.Usage.OutputTokens, "total_tokens", s.TotalTokens)
	emit(DoneEvent(s.Usage, s.TotalTokens, s.Truncated))
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
		// Dùng breaker của LƯỢT CHẠY NÀY (s.loopBreaker), không phải instance
		// chia sẻ toàn process — xem comment trong Engine.Run.
		if s.loopBreaker != nil {
			last := s.LastAssistant()
			if last != nil {
				for _, tc := range last.ToolCalls {
					if err := s.loopBreaker.Record(tc.Name, tc.Args); err != nil {
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
	case NodeInterrupt:
		return NodeEnd, nil
	default:
		return NodeEnd, fmt.Errorf("engine: unknown node %q", node)
	}
}

// Provider returns the engine's LLM provider (for health checks, etc.)
func (e *Engine) Provider() provider.Provider { return e.prov }

// Registry returns the engine's tool registry (for health checks, etc.)
func (e *Engine) Registry() *tools.Registry { return e.registry }
