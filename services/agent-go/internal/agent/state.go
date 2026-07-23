// Package agent chứa "engine" tự dựng: state machine chạy vòng
// recall→plan→model→route→tools→reflect→summarize→extract (thay LangGraph).
package agent

import "github.com/ai-agent-tut/agent-go/internal/provider"

// NodeID định danh một node trong đồ thị engine.
type NodeID string

const (
	NodeRecall    NodeID = "recall"
	NodePlan      NodeID = "plan"
	NodeModel     NodeID = "model"
	NodeTools     NodeID = "tools"
	NodeReflect   NodeID = "reflect"
	NodeSummarize NodeID = "summarize"
	NodeExtract   NodeID = "extract"
	NodeInterrupt NodeID = "interrupt"
	NodeEnd       NodeID = "end"
)

// RunInput là đầu vào cho một lượt chạy agent.
type RunInput struct {
	ConversationID string
	History        []provider.Message
	UserMessage    string
	Provider       string
	MaxSteps       int
}

// State là trạng thái xuyên suốt một lượt chạy (working memory).
type State struct {
	Messages []provider.Message // history + user + assistant + tool results của lượt
	Step     int
	MaxSteps int
	Usage    provider.Usage
	Done     bool
}

// lastAssistant trả message assistant gần nhất (nil nếu chưa có).
func (s *State) lastAssistant() *provider.Message {
	for i := len(s.Messages) - 1; i >= 0; i-- {
		if s.Messages[i].Role == provider.RoleAssistant {
			return &s.Messages[i]
		}
	}
	return nil
}
