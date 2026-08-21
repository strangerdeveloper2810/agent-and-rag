package orchestrator

import (
	"context"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/agent"
	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/tools"
)

// newFakeEngine tạo engine với FakeProvider cho test.
func newFakeEngine(name string, chunks ...provider.StreamChunk) *agent.Engine {
	prov := provider.NewFake(chunks...)
	reg := tools.NewRegistry()
	reg.Register(tools.NewEchoTool())
	return agent.NewEngine(prov, reg)
}

func TestOrchestrator_RouteByKeyword(t *testing.T) {
	orch := New()

	// General agent — keyword: "chat", "help", "note"
	genEngine := newFakeEngine("general",
		provider.StreamChunk{Kind: provider.ChunkText, Text: "General agent here."},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)
	orch.Register(&AgentSpec{
		Name:            "general",
		Description:     "Handles general chat and daily tasks",
		Engine:          genEngine,
		TriggerKeywords: []string{"chat", "help", "note", "nhật ký"},
		SystemPrompt:    "You are a general assistant.",
	})

	// Code agent — keyword: "code", "bug", "debug", "git"
	codeEngine := newFakeEngine("code",
		provider.StreamChunk{Kind: provider.ChunkText, Text: "Code agent here."},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)
	orch.Register(&AgentSpec{
		Name:            "code",
		Description:     "Handles code analysis, debugging, git operations",
		Engine:          codeEngine,
		TriggerKeywords: []string{"code", "bug", "debug", "git", "lỗi", "fix"},
		SystemPrompt:    "You are a code review expert.",
	})

	tests := []struct {
		name      string
		input     string
		wantAgent string
	}{
		{"code keyword bug", "tìm bug trong code này", "code"},
		{"code keyword fix", "fix lỗi giúp tôi", "code"},
		{"code keyword git", "git log của repo", "code"},
		{"general keyword chat", "chúng ta chat đi", "general"},
		{"general keyword help", "help tôi với", "general"},
		{"no keyword → default", "hôm nay trời đẹp quá", "general"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := orch.route(tt.input)
			if spec == nil {
				t.Fatal("route returned nil")
			}
			if spec.Name != tt.wantAgent {
				t.Errorf("route(%q) = %q, want %q", tt.input, spec.Name, tt.wantAgent)
			}
		})
	}
}

// TestOrchestrator_RouteWordBoundary khoá lỗi false-positive của matching thô:
// trước fix route() dùng strings.Contains nên keyword "go" của agent code khớp
// cả "golang", "mongo", "django", "google", "ngoài"; "test" khớp "latest" nên
// mọi câu hỏi có "latest" bị agent code cướp trước agent research.
func TestOrchestrator_RouteWordBoundary(t *testing.T) {
	orch := New()

	orch.Register(&AgentSpec{
		Name:            "general",
		Engine:          newFakeEngine("general"),
		TriggerKeywords: []string{},
	})
	orch.Register(&AgentSpec{
		Name:            "code",
		Engine:          newFakeEngine("code"),
		TriggerKeywords: []string{"code", "go", "test", "bug"},
	})
	orch.Register(&AgentSpec{
		Name:            "research",
		Engine:          newFakeEngine("research"),
		TriggerKeywords: []string{"latest", "tin tức", "giải thích"},
	})

	tests := []struct {
		name      string
		input     string
		wantAgent string
	}{
		{"'go' riêng vẫn khớp code", "viết cho tôi hàm go", "code"},
		{"'mongo' KHÔNG khớp 'go'", "cấu hình mongo replica set thế nào", "general"},
		{"'django' KHÔNG khớp 'go'", "django có gì hay", "general"},
		{"'google' KHÔNG khớp 'go'", "google vừa ra mắt gì", "general"},
		{"'ngoài' KHÔNG khớp 'go'", "ngoài ra còn cách nào khác", "general"},
		{"'latest' về research, không bị 'test' cướp", "latest AI news", "research"},
		{"'contest' KHÔNG khớp 'test'", "contest tuần này có gì", "general"},
		{"'debug' KHÔNG khớp 'bug' (word riêng)", "debug hộ tôi", "general"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := orch.route(tt.input)
			if spec == nil {
				t.Fatal("route returned nil")
			}
			if spec.Name != tt.wantAgent {
				t.Errorf("route(%q) = %q, want %q", tt.input, spec.Name, tt.wantAgent)
			}
		})
	}
}

// capturingProvider ghi lại GenerateRequest để assert system prompt THỰC SỰ
// được gửi tới LLM (không chỉ được gán vào struct).
type capturingProvider struct {
	LastRequest provider.GenerateRequest
}

func (p *capturingProvider) Name() string { return "capturing" }

func (p *capturingProvider) Generate(ctx context.Context, req provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
	p.LastRequest = req
	ch := make(chan provider.StreamChunk, 2)
	ch <- provider.StreamChunk{Kind: provider.ChunkText, Text: "ok"}
	ch <- provider.StreamChunk{Kind: provider.ChunkDone}
	close(ch)
	return ch, nil
}

func newCapturingEngine() (*agent.Engine, *capturingProvider) {
	p := &capturingProvider{}
	reg := tools.NewRegistry()
	reg.Register(tools.NewEchoTool())
	return agent.NewEngine(p, reg), p
}

// TestOrchestrator_RegisterAppliesSystemPrompt khoá bug dead code: field
// AgentSpec.SystemPrompt được gán ở cmd/server/main.go nhưng orchestrator
// không hàm nào đọc tới, nên prompt riêng của agent (vd 39 dòng quy trình của
// research agent) chưa bao giờ tới LLM. Test chạy qua Orchestrator.Run và
// kiểm chính request gửi cho provider.
func TestOrchestrator_RegisterAppliesSystemPrompt(t *testing.T) {
	orch := New()
	eng, cap := newCapturingEngine()

	const want = "[BẠN LÀ RESEARCH AGENT] quy trình nghiên cứu riêng"
	orch.Register(&AgentSpec{
		Name:         "research",
		Engine:       eng,
		SystemPrompt: want,
	})

	if _, err := orch.Run(context.Background(), agent.RunInput{UserMessage: "hi", MaxSteps: 2}, func(agent.Event) {}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := cap.LastRequest.System; got != want {
		t.Errorf("system prompt gửi cho LLM = %q, want %q", got, want)
	}
}

// SystemPrompt rỗng không được xoá prompt đã set sẵn trên engine (cmd/jarvis và
// một số nơi set trực tiếp qua Engine.SetSystemPrompt).
func TestOrchestrator_RegisterEmptySystemPromptKeepsExisting(t *testing.T) {
	orch := New()
	eng, cap := newCapturingEngine()
	eng.SetSystemPrompt("prompt đã set sẵn")

	orch.Register(&AgentSpec{Name: "general", Engine: eng})

	if _, err := orch.Run(context.Background(), agent.RunInput{UserMessage: "hi", MaxSteps: 2}, func(agent.Event) {}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := cap.LastRequest.System; got != "prompt đã set sẵn" {
		t.Errorf("system prompt gửi cho LLM = %q, want giữ nguyên prompt đã set sẵn", got)
	}
}

func TestOrchestrator_Run(t *testing.T) {
	orch := New()

	genEngine := newFakeEngine("general",
		provider.StreamChunk{Kind: provider.ChunkText, Text: "Xin chào!"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)
	orch.Register(&AgentSpec{
		Name:            "general",
		Engine:          genEngine,
		TriggerKeywords: []string{"hello"},
	})

	var events []agent.Event
	emit := func(e agent.Event) { events = append(events, e) }

	input := agent.RunInput{
		UserMessage: "hello",
		MaxSteps:    5,
	}

	_, err := orch.Run(context.Background(), input, emit)
	if err != nil {
		t.Fatal(err)
	}

	// Should have agent event + text + done
	foundAgent := false
	for _, e := range events {
		if e.Type == "agent" {
			foundAgent = true
			if e.Node != "general" {
				t.Errorf("agent event Node = %q, want general", e.Node)
			}
		}
	}
	if !foundAgent {
		t.Error("expected agent event indicating which agent ran")
	}
}

func TestOrchestrator_ListAgents(t *testing.T) {
	orch := New()

	orch.Register(&AgentSpec{
		Name:   "a",
		Engine: newFakeEngine("a", provider.StreamChunk{Kind: provider.ChunkDone}),
	})
	orch.Register(&AgentSpec{
		Name:   "b",
		Engine: newFakeEngine("b", provider.StreamChunk{Kind: provider.ChunkDone}),
	})

	list := orch.ListAgents()
	if len(list) != 2 {
		t.Fatalf("len = %d, want 2", len(list))
	}
	if list[0].Name != "a" || list[1].Name != "b" {
		t.Errorf("order wrong: %v", list)
	}
}

func TestExtractRoutableText(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantText  string
		wantReply bool
	}{
		{
			name:      "format Q:/A: → chỉ lấy phần sau A:",
			input:     "Q: Bạn muốn tập trung tìm hiểu phần nào của repo này trước?\nA: Core AI/RAG Logic",
			wantText:  "Core AI/RAG Logic",
			wantReply: true,
		},
		{
			name:      "câu bình thường không có format Q:/A: → giữ nguyên",
			input:     "đi sâu vào services-go của repo",
			wantText:  "đi sâu vào services-go của repo",
			wantReply: false,
		},
		{
			name:      "chỉ có A: không có Q: ở đầu → không coi là reply",
			input:     "A: là một chữ cái, không liên quan",
			wantText:  "A: là một chữ cái, không liên quan",
			wantReply: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotText, gotReply := extractRoutableText(tt.input)
			if gotText != tt.wantText || gotReply != tt.wantReply {
				t.Errorf("extractRoutableText(%q) = (%q, %v), want (%q, %v)",
					tt.input, gotText, gotReply, tt.wantText, tt.wantReply)
			}
		})
	}
}
