// Package orchestrator implements multi-agent orchestration.
// Một Orchestrator quản lý N specialized engines, mỗi engine = 1 ReAct loop.
// IntentRouter phân loại input → chọn agent. HandoffManager cho agent-to-agent.
package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/ai-agent-tut/agent-go/internal/agent"
	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// AgentSpec định nghĩa một specialized agent trong orchestrator.
type AgentSpec struct {
	Name            string          // "general", "code", "research"
	Description     string          // Mô tả cho intent classification
	Engine          *agent.Engine   // Engine ReAct (GIỮ NGUYÊN từ P2)
	TriggerKeywords []string        // Keyword để router chọn agent này (không cần LLM)
	SystemPrompt    string          // Prompt RIÊNG cho agent này (merge với base)
}

// Orchestrator quản lý nhiều engine, route request đến đúng agent.
type Orchestrator struct {
	agents    map[string]*AgentSpec // name → spec
	order     []string              // thứ tự đăng ký (ưu tiên)
	defaultAgent string             // fallback agent name
}

// New tạo Orchestrator rỗng.
func New() *Orchestrator {
	return &Orchestrator{
		agents: make(map[string]*AgentSpec),
	}
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
}

// HandoffResult là kết quả từ agent nhận.
type HandoffResult struct {
	Agent  string
	Result string
	Usage  provider.Usage
}

// Delegate chuyển task từ agent A → agent B và chạy agent B.
// Dùng khi một agent cần chuyên môn của agent khác.
func (o *Orchestrator) Delegate(ctx context.Context, req HandoffRequest) (*HandoffResult, error) {
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
