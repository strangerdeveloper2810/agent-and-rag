// Package orchestrator implements multi-agent orchestration.
// Một Orchestrator quản lý N specialized engines, mỗi engine = 1 ReAct loop.
// IntentRouter phân loại input → chọn agent. HandoffManager cho agent-to-agent.
package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/agent"
	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// AgentSpec định nghĩa một specialized agent trong orchestrator.
type AgentSpec struct {
	Name            string        // "general", "code", "research"
	Description     string        // Mô tả cho intent classification
	Engine          *agent.Engine // Engine ReAct (GIỮ NGUYÊN từ P2)
	TriggerKeywords []string      // Keyword để router chọn agent này (không cần LLM)
	// SystemPrompt là prompt RIÊNG của agent này, được áp vào Engine trong
	// Register(). Caller tự quyết định có nối với base prompt hay không (vd
	// cmd/server/main.go truyền BuildSystemPrompt(...) + phần riêng). Để rỗng
	// nếu muốn giữ prompt đã set sẵn trên Engine.
	SystemPrompt string
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

	stickyAgent map[string]stickyEntry // conversationID → agent đang xử lý hội thoại
	stickyMu    sync.RWMutex
}

// stickyEntry ghi lại agent đang "sở hữu" một hội thoại, để reply cho
// ask_user (format Q:/A:) không bị route lại dựa trên keyword tình cờ xuất
// hiện trong câu trả lời của user.
type stickyEntry struct {
	agentName string
	lastUsed  time.Time
}

// stickyAgentTTL: entry cũ hơn ngưỡng này bị coi như chưa có, route lại từ
// đầu — tránh 1 hội thoại bỏ dở nhiều ngày trước "kẹt cứng" agent cũ mãi mãi.
// Không cần sweep định kỳ: TTL tự áp dụng khi ĐỌC lại entry (xem stickyAgentFor).
const stickyAgentTTL = 24 * time.Hour

// New tạo Orchestrator rỗng.
func New() *Orchestrator {
	return &Orchestrator{
		agents:             make(map[string]*AgentSpec),
		maxDelegationDepth: defaultMaxDelegationDepth,
		stickyAgent:        make(map[string]stickyEntry),
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
//
// Nếu spec.SystemPrompt khác rỗng, nó được áp vào engine NGAY TẠI ĐÂY. Trước
// đây field này là dead code: nó được gán ở cmd/server/main.go nhưng không
// hàm nào trong orchestrator đọc tới, nên toàn bộ prompt riêng của agent
// (vd 39 dòng hướng dẫn quy trình của research agent) chưa bao giờ tới LLM —
// agent research chạy y hệt prompt chung. Áp ở Register (một lần, lúc wiring)
// thay vì trong Run: Engine được chia sẻ giữa các request đồng thời, gọi
// SetSystemPrompt mỗi request sẽ là data race.
func (o *Orchestrator) Register(spec *AgentSpec) {
	name := spec.Name
	if _, exists := o.agents[name]; !exists {
		o.order = append(o.order, name)
	}
	o.agents[name] = spec
	if spec.SystemPrompt != "" && spec.Engine != nil {
		spec.Engine.SetSystemPrompt(spec.SystemPrompt)
	}
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
	routableText, isReply := extractRoutableText(in.UserMessage)

	var spec *AgentSpec
	// Reply cho ask_user PHẢI tiếp tục agent đang xử lý hội thoại — câu trả
	// lời của user (thường là chọn 1 option) không phải tín hiệu đổi chủ đề,
	// dù tình cờ chứa keyword của agent khác.
	if isReply {
		if sticky, ok := o.stickyAgentFor(in.ConversationID); ok {
			spec = sticky
		}
	}
	if spec == nil {
		spec = o.route(routableText)
	}
	o.setStickyAgent(in.ConversationID, spec.Name)

	slog.Info("orchestrator: routed", "agent", spec.Name, "input_preview", truncate(in.UserMessage, 100))

	emit(agent.Event{
		Type: "agent",
		Node: spec.Name,
	})

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
			if matchTrigger(lower, kw) {
				return spec
			}
		}
	}

	return o.agents[o.defaultAgent]
}

// stickyAgentFor trả về agent đã ghi cho conversationID này, nếu còn hiệu
// lực (chưa quá TTL và agent đó còn đăng ký trong orchestrator).
func (o *Orchestrator) stickyAgentFor(conversationID string) (*AgentSpec, bool) {
	if conversationID == "" {
		return nil, false
	}
	o.stickyMu.RLock()
	entry, ok := o.stickyAgent[conversationID]
	o.stickyMu.RUnlock()
	if !ok || time.Since(entry.lastUsed) > stickyAgentTTL {
		return nil, false
	}
	spec, exists := o.agents[entry.agentName]
	if !exists {
		return nil, false
	}
	return spec, true
}

// setStickyAgent ghi lại agent vừa xử lý hội thoại này.
func (o *Orchestrator) setStickyAgent(conversationID, agentName string) {
	if conversationID == "" {
		return
	}
	o.stickyMu.Lock()
	o.stickyAgent[conversationID] = stickyEntry{agentName: agentName, lastUsed: time.Now()}
	o.stickyMu.Unlock()
}

// qaReplyRe khớp input dạng "Q: <câu hỏi>\nA: <câu trả lời>" — format mà FE
// gửi lại khi user trả lời tool ask_user. Bắt buộc bắt đầu bằng "Q:" để
// tránh false positive với câu user gõ tự nhiên có chứa "A: " ở đâu đó.
var qaReplyRe = regexp.MustCompile(`(?s)^Q:\s*.*?\nA:\s*(.*)$`)

// extractRoutableText trả về phần nên dùng để match keyword routing.
//
// Nếu input là reply cho ask_user (dạng "Q: .../A: ..."), CHỈ lấy phần sau
// "A: " — bỏ phần câu hỏi do chính JARVIS đặt ra. Không làm vậy thì keyword
// trong câu hỏi JARVIS tự sinh (vd "tìm hiểu", "giải thích") tự khớp trigger
// của agent khác, khiến JARVIS tự lạc đề khỏi agent đang xử lý hội thoại.
func extractRoutableText(input string) (text string, isReply bool) {
	m := qaReplyRe.FindStringSubmatch(strings.TrimSpace(input))
	if m == nil {
		return input, false
	}
	return strings.TrimSpace(m[1]), true
}

// asciiWordRe nhận diện keyword ASCII đơn từ (chữ cái/số, không khoảng trắng).
var asciiWordRe = regexp.MustCompile(`^[a-z0-9]+$`)

// triggerRegexCache cache regex word-boundary theo keyword, tạo lazy khi gặp
// lần đầu (TriggerKeywords do caller cung cấp lúc Register nên không biết
// trước ở package init như internal/tools/filter.go).
var (
	triggerRegexMu    sync.RWMutex
	triggerRegexCache = map[string]*regexp.Regexp{}
)

// matchTrigger khớp trigger keyword với input: keyword ASCII đơn từ dùng word
// boundary, còn lại (tiếng Việt có dấu, cụm nhiều từ) dùng substring.
//
// Trước fix, route() dùng strings.Contains thô cho MỌI keyword nên keyword "go"
// của agent code khớp cả "golang", "goroutine", "mongo", "django", "google",
// "algorithm"; "test" khớp "latest" (khiến mọi câu hỏi có "latest" bị agent
// code cướp trước research); "bug" khớp "debug". internal/tools/filter.go đã
// fix đúng lớp lỗi này từ trước nhưng orchestrator thì chưa — đây là chỗ đồng
// bộ lại hai nơi.
func matchTrigger(s, kw string) bool {
	if !asciiWordRe.MatchString(kw) {
		return strings.Contains(s, kw)
	}

	triggerRegexMu.RLock()
	re, ok := triggerRegexCache[kw]
	triggerRegexMu.RUnlock()
	if !ok {
		re = regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(kw) + `\b`)
		triggerRegexMu.Lock()
		triggerRegexCache[kw] = re
		triggerRegexMu.Unlock()
	}
	return re.MatchString(s)
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
