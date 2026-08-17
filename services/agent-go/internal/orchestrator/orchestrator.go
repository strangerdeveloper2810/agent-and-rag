// Package orchestrator implements multi-agent orchestration.
// Một Orchestrator quản lý N specialized engines, mỗi engine = 1 ReAct loop.
// IntentRouter phân loại input → chọn agent. HandoffManager cho agent-to-agent.
package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ai-agent-tut/agent-go/internal/agent"
	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// AgentSpec định nghĩa một specialized agent trong orchestrator.
type AgentSpec struct {
	Name            string        // "general", "code", "research"
	Description     string        // Mô tả cho intent classification
	Engine          *agent.Engine // Engine ReAct (GIỮ NGUYÊN từ P2)
	TriggerKeywords []string      // Keyword để router chọn agent này (không cần LLM)
	SystemPrompt    string        // Prompt RIÊNG cho agent này (merge với base)
}

// defaultMaxDelegationDepth chặn handoff đệ quy vô hạn (A→B→A→B→...) khi chưa
// gọi SetMaxDelegationDepth. 0 từ HandoffRequest.Depth nghĩa là handoff gốc.
const defaultMaxDelegationDepth = 4

// Orchestrator quản lý nhiều engine, route request đến đúng agent.
type Orchestrator struct {
	agents             map[string]*AgentSpec // name → spec
	order              []string              // thứ tự đăng ký (ưu tiên)
	defaultAgent       string                // fallback agent name
	maxDelegationDepth int                   // chốt an toàn cho Delegate() đệ quy
}

// New tạo Orchestrator rỗng.
func New() *Orchestrator {
	return &Orchestrator{
		agents:             make(map[string]*AgentSpec),
		maxDelegationDepth: defaultMaxDelegationDepth,
	}
}

// SetMaxDelegationDepth đặt số lần handoff liên tiếp tối đa Delegate() cho
// phép (A→B→C→...). n <= 0 → dùng lại defaultMaxDelegationDepth.
func (o *Orchestrator) SetMaxDelegationDepth(n int) {
	if n <= 0 {
		n = defaultMaxDelegationDepth
	}
	o.maxDelegationDepth = n
}

// Register thêm một agent vào orchestrator.
// Agent đăng ký trước có độ ưu tiên cao hơn trong keyword matching.
func (o *Orchestrator) Register(spec *AgentSpec) {
	name := spec.Name
	if _, exists := o.agents[name]; !exists {
		o.order = append(o.order, name)
	}
	o.agents[name] = spec
	if o.defaultAgent == "" {
		o.defaultAgent = name // agent đầu tiên là default
	}
}

// SetDefault đặt default agent (fallback khi không match keyword).
func (o *Orchestrator) SetDefault(name string) error {
	if _, ok := o.agents[name]; !ok {
		return fmt.Errorf("orchestrator: agent %q not registered", name)
	}
	o.defaultAgent = name
	return nil
}

// Run xử lý user input: route → chọn agent → run engine.
// Giữ nguyên signature giống Engine.Run để dễ swap.
func (o *Orchestrator) Run(ctx context.Context, in agent.RunInput, emit agent.EmitFunc) (provider.Usage, error) {
	// 1. Route: chọn agent dựa trên keyword matching
	spec := o.route(in.UserMessage)
	slog.Info("orchestrator: routed", "agent", spec.Name, "input_preview", truncate(in.UserMessage, 100))

	// 2. Báo client biết agent nào đang xử lý
	emit(agent.Event{
		Type: "agent",
		Node: spec.Name,
	})

	// 3. Chạy engine của agent được chọn (GIỮ NGUYÊN Engine.Run)
	return spec.Engine.Run(ctx, in, emit)
}

// route chọn agent dựa trên keyword matching.
// Duyệt theo thứ tự đăng ký → agent đầu tiên match keyword được chọn.
// Nếu không match → default agent.
func (o *Orchestrator) route(input string) *AgentSpec {
	lower := strings.ToLower(input)

	// Keyword matching (theo thứ tự đăng ký = ưu tiên)
	for _, name := range o.order {
		spec := o.agents[name]
		for _, kw := range spec.TriggerKeywords {
			if strings.Contains(lower, kw) {
				return spec
			}
		}
	}

	return o.agents[o.defaultAgent]
}

// GetAgent trả về agent spec theo tên.
func (o *Orchestrator) GetAgent(name string) *AgentSpec {
	return o.agents[name]
}

// ListAgents trả về danh sách tất cả agent specs.
func (o *Orchestrator) ListAgents() []*AgentSpec {
	out := make([]*AgentSpec, 0, len(o.order))
	for _, name := range o.order {
		out = append(out, o.agents[name])
	}
	return out
}

// HandoffRequest mô tả một yêu cầu delegate từ agent A → agent B.
type HandoffRequest struct {
	From    string // agent gửi
	To      string // agent nhận
	Context string // context để agent nhận hiểu task
	Task    string // task cụ thể

	// Depth = số lần handoff đã đi qua trước request này (0 = handoff gốc).
	// Nếu agent nhận (spec.To) tự gọi Delegate tiếp, nó PHẢI truyền Depth+1 —
	// đây là field duy nhất chống đệ quy vô hạn A→B→A→B→..., Delegate không
	// tự suy luận được độ sâu vì không giữ call-stack giữa các lượt.
	Depth int
}

// DelegationDepthExceededError báo Delegate bị chặn vì chuỗi handoff vượt
// giới hạn cấu hình (SetMaxDelegationDepth) — fail loud thay vì lặp vô hạn.
type DelegationDepthExceededError struct {
	From, To string
	Depth    int
	Max      int
}

func (e *DelegationDepthExceededError) Error() string {
	return fmt.Sprintf("orchestrator: delegation depth %d exceeds max %d (handoff %s→%s)",
		e.Depth, e.Max, e.From, e.To)
}

// HandoffResult là kết quả từ agent nhận.
type HandoffResult struct {
	Agent  string
	Result string
	Usage  provider.Usage
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// Delegate chuyển task từ agent A → agent B và chạy agent B.
// Dùng khi một agent cần chuyên môn của agent khác.
func (o *Orchestrator) Delegate(ctx context.Context, req HandoffRequest) (*HandoffResult, error) {
	if req.Depth >= o.maxDelegationDepth {
		return nil, &DelegationDepthExceededError{From: req.From, To: req.To, Depth: req.Depth, Max: o.maxDelegationDepth}
	}

	spec := o.agents[req.To]
	if spec == nil {
		return nil, fmt.Errorf("orchestrator: agent %q not found for handoff from %q", req.To, req.From)
	}

	input := agent.RunInput{
		UserMessage: req.Task,
		History: []provider.Message{
			{Role: provider.RoleSystem, Content: req.Context},
		},
		MaxSteps: 8,
	}

	var finalText string
	emit := func(e agent.Event) {
		if e.Type == "text" {
			finalText += e.Text
		}
	}

	usage, err := spec.Engine.Run(ctx, input, emit)
	if err != nil {
		return nil, fmt.Errorf("orchestrator: handoff %s→%s: %w", req.From, req.To, err)
	}

	return &HandoffResult{
		Agent:  req.To,
		Result: finalText,
		Usage:  usage,
	}, nil
}
