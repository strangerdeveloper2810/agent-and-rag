// Package agent chứa "engine" tự dựng: state machine chạy vòng
// recall→plan→model→route→tools→reflect→summarize→extract (thay LangGraph).
//
// Kiến trúc: mỗi node là một hàm thuần Node func(ctx, state, emit) (nextNode, error).
// Router là hàm thuần route(state) NodeID — quyết định node kế tiếp.
// Engine chạy vòng lặp: dispatch node → emit event → check next → lặp hoặc END.
package agent

import (
	"context"

	"github.com/ai-agent-tut/agent-go/internal/guardrails"
	"github.com/ai-agent-tut/agent-go/internal/provider"
)

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

// Node là hàm thực thi một node trong engine.
// Nhận ctx (cancel/timeout), con trỏ State (đọc/ghi), và emit (phát event).
// Trả về NodeID tiếp theo (NodeEnd để dừng) hoặc error.
type Node func(ctx context.Context, s *State, emit EmitFunc) (NodeID, error)

// RunInput là đầu vào cho một lượt chạy agent.
type RunInput struct {
	ConversationID string
	History        []provider.Message
	UserMessage    string
	Attachments    []provider.Attachment // image/file attachments (multimodal)
	Provider       string
	MaxSteps       int

	// Lang là ngôn ngữ UI người dùng chọn cho LƯỢT NÀY (vd "en", "vi"). Rỗng =
	// không chỉ định → agent giữ hành vi mặc định (tiếng Việt) đã bake sẵn
	// trong system prompt cacheable. Truyền qua RunInput (thay vì đổi system
	// prompt của Engine) vì Engine dùng chung cho mọi request đồng thời —
	// xem comment ở cmd/server/main.go và node_model.go.
	Lang string
}

// Observation là kết quả của một lần chạy tool — lưu trong Scratchpad.
// Mỗi Observation tương ứng với 1 provider.ToolCall đã thực thi.
type Observation struct {
	CallID string // ID của tool_call (khớp với provider.ToolCall.ID)
	Name   string // tên tool đã chạy
	Output string // kết quả dạng text (đưa lại cho LLM)
	Error  string // nếu có lỗi khi chạy tool (rỗng = ok)
}

// Interrupt mô tả một điểm dừng chờ xác nhận từ người dùng (HITL).
// Engine phát Event{Type:"interrupt"} rồi DỪNG; client gọi /chat/resume để tiếp tục.
type Interrupt struct {
	Reason string // lý do: "confirm_destructive", "confirm_write", ...
	Tool   string // tên tool bị chặn
	Args   string // args của tool call (JSON string, để UI hiển thị)
}

// State là trạng thái xuyên suốt một lượt chạy (working memory).
type State struct {
	// Messages chứa toàn bộ hội thoại trong lượt: history + user message +
	// assistant messages (có thể có tool_calls) + tool result messages.
	// Đây là thứ sẽ gửi cho LLM ở mỗi lần gọi model.
	Messages []provider.Message

	// Scratchpad lưu kết quả thô của mỗi tool đã chạy trong lượt (để debug/audit).
	// Sau khi tools chạy, kết quả cũng được append vào Messages dưới dạng role=tool.
	Scratchpad []Observation

	Step     int
	MaxSteps int
	Usage    provider.Usage // cumulative input/output tokens across all model calls
	Done     bool

	// TotalTokens là tổng số token (input+output) tích lũy qua tất cả các bước.
	// Bằng Usage.InputTokens + Usage.OutputTokens, được đồng bộ sau mỗi model call.
	TotalTokens int

	// TrimmedTokens counts how many tokens were trimmed from context
	// during this run (for observability).
	TrimmedTokens int

	// ToolOutputRunesUsed đếm số ký tự tool output đã đưa vào context, CỘNG DỒN
	// qua tất cả bước của lượt chạy này — xem applyToolOutputBudget. Không đếm
	// theo byte vì text tiếng Việt multi-byte sẽ làm sai lệch giới hạn.
	ToolOutputRunesUsed int

	// Truncated = true khi model call gần nhất dừng vì chạm giới hạn output
	// token (finish reason "length"). Client dùng cờ này để mời user bấm
	// "Tiếp tục".
	Truncated bool

	// Interrupt != nil khi engine dừng chờ HITL. Lúc này State có thể được
	// serialize/lưu lại để resume sau.
	Interrupt *Interrupt

	// Plan chứa danh sách các bước (steps) được sinh ra bởi node plan.
	// PlanStep là chỉ số bước hiện tại (0-based). Khi PlanStep >= len(Plan), plan đã hoàn thành.
	// Plan rỗng nghĩa là request đơn giản, không cần plan.
	Plan     []string
	PlanStep int

	// activatedSkills tracks which skills have been activated during this run.
	// Used to prevent re-activation of the same skill within one conversation.
	activatedSkills map[string]bool

	// loopBreaker phát hiện vòng lặp tool (cùng tool + cùng args gọi liên tiếp)
	// TRONG PHẠM VI lượt chạy này. Engine.Run tạo mới mỗi lượt — xem comment ở
	// guardrails.CircuitBreaker về lý do không dùng instance chia sẻ.
	loopBreaker *guardrails.CircuitBreaker

	// RecalledMemories holds the long-term memories found by the recall node
	// for the current user turn (formatted as "key: value" strings). nodeModel
	// weaves these into the system prompt sent to the LLM — without this,
	// recall results only reached the SSE stream for UI display and were
	// never actually used by the model to answer the request.
	RecalledMemories []string

	// Lang là ngôn ngữ UI người dùng chọn cho lượt này — copy từ RunInput.Lang.
	// nodeModel dùng field này để ghi đè chỉ dẫn ngôn ngữ trong system prompt
	// riêng cho lượt chạy này (per-request), không đụng vào systemPrompt tĩnh
	// dùng chung giữa các request khác.
	Lang string
}

// newState khởi tạo State từ RunInput.
// Append user message vào history và đánh dấu bắt đầu lượt.
// Nếu có attachments: file content được nối vào Content dạng text,
// image được giữ trong Message.Attachments để adapter xử lý multimodal.
func newState(in RunInput) *State {
	messages := make([]provider.Message, 0, len(in.History)+1)
	messages = append(messages, in.History...)

	// Build user message content with any file attachment text.
	content := in.UserMessage
	var imageAttachments []provider.Attachment
	for _, att := range in.Attachments {
		switch att.Type {
		case "file":
			content += "\n\n[File: " + att.Name + "]\n" + att.Data
		case "image":
			imageAttachments = append(imageAttachments, att)
			content += "\n[Image attached: " + att.Name + "]"
		}
	}

	messages = append(messages, provider.Message{
		Role:        provider.RoleUser,
		Content:     content,
		Attachments: imageAttachments,
	})

	maxSteps := in.MaxSteps
	if maxSteps <= 0 {
		maxSteps = 12 // mặc định an toàn
	}

	return &State{
		Messages: messages,
		MaxSteps: maxSteps,
		Lang:     in.Lang,
	}
}

// LastAssistant trả về message cuối cùng có Role=assistant và con trỏ của nó
// trong Messages (để có thể sửa — vd node model gom stream text vào Content).
// Trả nil nếu chưa có assistant message nào.
func (s *State) LastAssistant() *provider.Message {
	for i := len(s.Messages) - 1; i >= 0; i-- {
		if s.Messages[i].Role == provider.RoleAssistant {
			return &s.Messages[i]
		}
	}
	return nil
}

// LastUserContent trả về nội dung message user MỚI NHẤT (duyệt ngược) — tức
// câu người dùng đang hỏi ở lượt này, không phải câu đầu cuộc hội thoại.
//
// Đây là câu hỏi đúng để lọc tool, match skill và chọn thinking level. Trước
// khi có helper này, node_model và node_plan tự duyệt XUÔI rồi break, nên
// trong MỌI lượt chat có history chúng lấy câu hỏi CŨ NHẤT: tool được lọc
// theo intent của câu đã trả lời xong từ lâu, skill activate theo câu cũ.
// memory.RecallNode thì duyệt ngược (đúng) — sự bất đối xứng đó chính là bug.
// Giữ một helper duy nhất ở đây để 3 chỗ không thể lệch nhau lần nữa.
func (s *State) LastUserContent() string {
	for i := len(s.Messages) - 1; i >= 0; i-- {
		if s.Messages[i].Role == provider.RoleUser {
			return s.Messages[i].Content
		}
	}
	return ""
}

// AppendObservation thêm một observation vào Scratchpad và tạo message
// role=tool tương ứng trong Messages (để LLM thấy kết quả tool).
func (s *State) AppendObservation(obs Observation) {
	s.Scratchpad = append(s.Scratchpad, obs)
	content := obs.Output
	if obs.Error != "" {
		content = "ERROR: " + obs.Error
	}
	s.Messages = append(s.Messages, provider.Message{
		Role:       provider.RoleTool,
		ToolCallID: obs.CallID,
		Content:    content,
	})
}

// EmitFunc là callback engine dùng để phát Event ra ngoài (→ SSE writer).
type EmitFunc func(Event)

// Runner is the interface that both Engine and Orchestrator implement.
// This allows the HTTP transport to accept either a single engine or a
// multi-agent orchestrator without code changes.
type Runner interface {
	Run(ctx context.Context, in RunInput, emit EmitFunc) (provider.Usage, error)
}

// compile-time checks
var _ Node = func(ctx context.Context, s *State, emit EmitFunc) (NodeID, error) {
	return NodeEnd, nil
}
var _ Runner = (*Engine)(nil)
