package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/skills"
	"github.com/ai-agent-tut/agent-go/internal/tools"
)

// fakeEngine là engine tối thiểu chỉ để test node model (có provider + registry).
type fakeEngine struct {
	ownerTenants     []string
	prov             provider.Provider
	registry         *tools.Registry
	maxOutputTokens  int
	maxContextTokens int
	fastModel        string
}

func (e *fakeEngine) getProvider() provider.Provider            { return e.prov }
func (e *fakeEngine) getRegistry() *tools.Registry              { return e.registry }
func (e *fakeEngine) getSystemPrompt() string                   { return "" }
func (e *fakeEngine) getMaxContextTokens() int                  { return e.maxContextTokens }
func (e *fakeEngine) getMaxOutputTokens() int                   { return e.maxOutputTokens }
func (e *fakeEngine) getOwnerTenants() []string                 { return e.ownerTenants }
func (e *fakeEngine) getDynamicThinking() DynamicThinkingConfig { return DynamicThinkingConfig{} }
func (e *fakeEngine) getSkillLoader() *skills.Loader            { return nil }
func (e *fakeEngine) getFastModel() string                      { return e.fastModel }

func TestNodeModel_TextOnly(t *testing.T) {
	// Kịch bản đơn giản nhất: LLM trả về text, không tool call.
	fake := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "Xin chào!"},
		provider.StreamChunk{Kind: provider.ChunkText, Text: " Tôi là AI."},
		provider.StreamChunk{Kind: provider.ChunkUsage, Usage: &provider.Usage{InputTokens: 10, OutputTokens: 5}},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)

	eng := &fakeEngine{prov: fake, registry: tools.NewRegistry()}
	s := newState(RunInput{
		UserMessage: "Hello",
		MaxSteps:    12,
	})

	var events []Event
	emit := func(e Event) { events = append(events, e) }

	next, err := nodeModel(context.Background(), eng, s, emit)
	if err != nil {
		t.Fatalf("nodeModel error: %v", err)
	}
	if next != NodeExtract {
		t.Fatalf("next = %q, want %q (no tool calls → extract memory)", next, NodeExtract)
	}

	// Kiểm tra events: phải có text, done.
	if len(events) < 1 {
		t.Fatal("expected at least 1 event")
	}
	texts := collectTexts(events)
	combined := strings.Join(texts, "")
	if combined != "Xin chào! Tôi là AI." {
		t.Errorf("combined text = %q, want %q", combined, "Xin chào! Tôi là AI.")
	}

	// Kiểm tra state: phải có assistant message, step=1, usage được cộng.
	if s.Step != 1 {
		t.Errorf("Step = %d, want 1", s.Step)
	}
	last := s.LastAssistant()
	if last == nil {
		t.Fatal("LastAssistant() = nil")
	}
	if last.Role != provider.RoleAssistant {
		t.Errorf("last.Role = %q, want assistant", last.Role)
	}
	if last.Content != "Xin chào! Tôi là AI." {
		t.Errorf("last.Content = %q", last.Content)
	}
	if s.Usage.InputTokens != 10 || s.Usage.OutputTokens != 5 {
		t.Errorf("Usage = %+v, want {10 5}", s.Usage)
	}
	if s.TotalTokens != 15 {
		t.Errorf("TotalTokens = %d, want 15", s.TotalTokens)
	}
}

func TestNodeModel_WithToolCall(t *testing.T) {
	// LLM trả về 1 tool_call → router phải trả về NodeTools.
	fake := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkToolCall, ToolCall: &provider.ToolCall{
			ID: "c1", Name: "echo", Args: nil,
		}},
		provider.StreamChunk{Kind: provider.ChunkUsage, Usage: &provider.Usage{InputTokens: 5, OutputTokens: 3}},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)

	eng := &fakeEngine{prov: fake, registry: tools.NewRegistry()}
	s := newState(RunInput{UserMessage: "echo test", MaxSteps: 12})

	next, err := nodeModel(context.Background(), eng, s, nilEmit)
	if err != nil {
		t.Fatalf("nodeModel error: %v", err)
	}
	if next != NodeTools {
		t.Fatalf("next = %q, want %q (tool call → tools)", next, NodeTools)
	}

	// Kiểm tra assistant message có tool call.
	last := s.LastAssistant()
	if last == nil || len(last.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %+v", last)
	}
	if last.ToolCalls[0].Name != "echo" {
		t.Errorf("tool name = %q, want echo", last.ToolCalls[0].Name)
	}
}

func TestNodeModel_ProviderError(t *testing.T) {
	// Provider trả về chunk lỗi → nodeModel phải trả về error.
	fake := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkError, Err: context.DeadlineExceeded},
	)

	eng := &fakeEngine{prov: fake, registry: tools.NewRegistry()}
	s := newState(RunInput{UserMessage: "test", MaxSteps: 12})

	_, err := nodeModel(context.Background(), eng, s, nilEmit)
	if err == nil {
		t.Fatal("expected error from provider, got nil")
	}
}

// capturingProvider ghi lại GenerateRequest cuối cùng nhận được — dùng để
// assert nội dung system prompt thực sự gửi cho LLM (provider.FakeProvider
// không giữ lại request).
type capturingProvider struct {
	chunks      []provider.StreamChunk
	LastRequest provider.GenerateRequest
}

func newCapturingProvider(chunks ...provider.StreamChunk) *capturingProvider {
	return &capturingProvider{chunks: chunks}
}

func (p *capturingProvider) Name() string { return "capturing" }

func (p *capturingProvider) Generate(ctx context.Context, req provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
	p.LastRequest = req
	ch := make(chan provider.StreamChunk, len(p.chunks))
	for _, c := range p.chunks {
		ch <- c
	}
	close(ch)
	return ch, nil
}

// TestNodeModel_InjectsRecalledMemoriesIntoSystemPrompt xác nhận fix P1:
// memory.RecallNode ghi kết quả recall vào s.RecalledMemories, và nodeModel
// PHẢI ghép nó vào system prompt thực sự gửi cho LLM — trước fix, RecallNode
// chỉ emit SSE MemoryEvent cho UI còn LLM luôn nhận BuildSystemPrompt(nil, ...)
// nên không bao giờ "nhớ" được gì khi trả lời.
func TestNodeModel_InjectsRecalledMemoriesIntoSystemPrompt(t *testing.T) {
	fake := newCapturingProvider(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "OK"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)

	eng := &fakeEngine{prov: fake, registry: tools.NewRegistry()}
	s := newState(RunInput{UserMessage: "tên tôi là gì?", MaxSteps: 12})
	s.RecalledMemories = []string{"user_name: Linh", "user_job: engineer"}

	_, err := nodeModel(context.Background(), eng, s, nilEmit)
	if err != nil {
		t.Fatalf("nodeModel error: %v", err)
	}

	sysPrompt := fake.LastRequest.System
	if !strings.Contains(sysPrompt, "user_name: Linh") {
		t.Errorf("system prompt = %q, want chứa user_name: Linh", sysPrompt)
	}
	if !strings.Contains(sysPrompt, "user_job: engineer") {
		t.Errorf("system prompt = %q, want chứa user_job: engineer", sysPrompt)
	}
	if !strings.Contains(sysPrompt, "[BỘ NHỚ]") {
		t.Errorf("system prompt = %q, want chứa section [BỘ NHỚ]", sysPrompt)
	}
}

// TestNodeModel_FiltersToolsByLatestUserMessage khoá đúng bug gốc: nodeModel
// từng lấy user message ĐẦU TIÊN trong history (duyệt xuôi + break) làm căn cứ
// lọc tool, match skill và chọn thinking level. Hệ quả: từ lượt chat thứ 2 trở
// đi, tool được lọc theo câu hỏi đã trả lời xong từ trước — câu hỏi hiện tại
// không hề ảnh hưởng tới tool được cấp.
//
// Dựng đúng tình huống thật: lượt 1 là câu chào (không intent gì), lượt 2 hỏi
// về tài liệu đã upload. Tool gửi cho LLM phải theo lượt 2.
func TestNodeModel_FiltersToolsByLatestUserMessage(t *testing.T) {
	reg := tools.NewRegistry()
	reg.Register(tools.NewEchoTool())
	reg.Register(tools.NewWebSearchTool(nil))
	reg.Register(tools.NewWebFetchTool(nil))
	reg.Register(tools.NewFileSearchTool(nil))
	reg.Register(tools.NewFileReadTool(nil))
	reg.Register(tools.NewShellTool(nil))
	reg.Register(tools.NewCalculatorTool())
	memStore := newFakeToolMemoryStore()
	reg.Register(tools.NewRecallMemoryTool(memStore))
	reg.Register(tools.NewSaveMemoryTool(memStore))
	reg.Register(tools.NewRAGSearchTool(nil, "test", "", nil, tools.RAGSearchConfig{}))

	fake := newCapturingProvider(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "OK"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)
	eng := &fakeEngine{prov: fake, registry: reg}

	s := newState(RunInput{
		History: []provider.Message{
			{Role: provider.RoleUser, Content: "xin chào"},
			{Role: provider.RoleAssistant, Content: "Chào sir, tôi có thể giúp gì?"},
		},
		UserMessage: "trong tài liệu tôi đã upload có nói gì về convention không?",
		MaxSteps:    12,
	})

	if _, err := nodeModel(context.Background(), eng, s, nilEmit); err != nil {
		t.Fatalf("nodeModel error: %v", err)
	}

	sent := make(map[string]bool, len(fake.LastRequest.Tools))
	for _, d := range fake.LastRequest.Tools {
		sent[d.Name] = true
	}

	// Câu hỏi mới nhắc "tài liệu"/"upload"/"convention" → phải được cấp rag.search.
	// Nếu code lấy "xin chào" (câu đầu) thì đây là nhánh no-intent, rag.search
	// bị loại và test này fail — đúng như hành vi trước fix.
	if !sent["rag.search"] {
		t.Errorf("tool gửi cho LLM = %v, want có rag.search (lọc theo câu hỏi MỚI về tài liệu)", sent)
	}
}

// TestNodeModel_LatestMessageDoesNotLeakRAGForGenericQuestion là mặt còn lại:
// lượt trước hỏi về tài liệu, lượt này hỏi kiến thức lập trình chung chung →
// KHÔNG được tiếp tục cấp rag.search chỉ vì câu cũ có nhắc tài liệu.
func TestNodeModel_LatestMessageDoesNotLeakRAGForGenericQuestion(t *testing.T) {
	reg := tools.NewRegistry()
	reg.Register(tools.NewEchoTool())
	reg.Register(tools.NewWebSearchTool(nil))
	reg.Register(tools.NewWebFetchTool(nil))
	reg.Register(tools.NewFileSearchTool(nil))
	reg.Register(tools.NewFileReadTool(nil))
	reg.Register(tools.NewShellTool(nil))
	reg.Register(tools.NewCalculatorTool())
	memStore := newFakeToolMemoryStore()
	reg.Register(tools.NewRecallMemoryTool(memStore))
	reg.Register(tools.NewSaveMemoryTool(memStore))
	reg.Register(tools.NewRAGSearchTool(nil, "test", "", nil, tools.RAGSearchConfig{}))

	fake := newCapturingProvider(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "OK"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)
	eng := &fakeEngine{prov: fake, registry: reg}

	s := newState(RunInput{
		History: []provider.Message{
			{Role: provider.RoleUser, Content: "trong tài liệu tôi upload có gì về convention?"},
			{Role: provider.RoleAssistant, Content: "Tài liệu nói rằng..."},
		},
		UserMessage: "Viết custom hook useMemo kết hợp useSelector của react-redux",
		MaxSteps:    12,
	})

	if _, err := nodeModel(context.Background(), eng, s, nilEmit); err != nil {
		t.Fatalf("nodeModel error: %v", err)
	}

	for _, d := range fake.LastRequest.Tools {
		if d.Name == "rag.search" {
			t.Errorf("câu hỏi lập trình chung chung không được cấp rag.search (rò rỉ từ câu hỏi cũ)")
		}
	}
}

// Không có memory nào được recall (RecalledMemories rỗng) → system prompt
// không được thêm section [BỘ NHỚ] (tránh noise/token thừa).
func TestNodeModel_NoRecalledMemories_NoMemorySection(t *testing.T) {
	fake := newCapturingProvider(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "OK"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)

	eng := &fakeEngine{prov: fake, registry: tools.NewRegistry()}
	s := newState(RunInput{UserMessage: "hello", MaxSteps: 12})

	_, err := nodeModel(context.Background(), eng, s, nilEmit)
	if err != nil {
		t.Fatalf("nodeModel error: %v", err)
	}

	if strings.Contains(fake.LastRequest.System, "[BỘ NHỚ]") {
		t.Errorf("system prompt = %q, không được chứa [BỘ NHỚ] khi không có memory nào", fake.LastRequest.System)
	}
}

// TestNodeModel_LangOverride khoá hành vi i18n per-request: getSystemPrompt()
// của fakeEngine trả về prompt mặc định tiếng Việt (giống Engine thật, được
// build 1 lần lúc wiring — xem cmd/server/main.go), nhưng khi RunInput.Lang =
// "en" cho lượt chạy hiện tại, nodeModel phải ghi đè bằng chỉ dẫn tiếng Anh mà
// KHÔNG cần Engine gọi SetSystemPrompt mỗi request (vốn không an toàn vì
// Engine dùng chung cho nhiều request đồng thời — xem orchestrator.Register).
func TestNodeModel_LangOverride(t *testing.T) {
	tests := []struct {
		name string
		lang string
		want string
	}{
		{name: "lang=en → ghi đè English", lang: "en", want: "ALWAYS respond in English"},
		{name: "lang=vi → không ghi đè", lang: "vi", want: ""},
		{name: "lang rỗng → không ghi đè", lang: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newCapturingProvider(
				provider.StreamChunk{Kind: provider.ChunkText, Text: "OK"},
				provider.StreamChunk{Kind: provider.ChunkDone},
			)
			eng := &fakeEngine{prov: fake, registry: tools.NewRegistry()}
			s := newState(RunInput{UserMessage: "hello", MaxSteps: 12, Lang: tt.lang})

			if _, err := nodeModel(context.Background(), eng, s, nilEmit); err != nil {
				t.Fatalf("nodeModel error: %v", err)
			}

			if tt.want == "" {
				if strings.Contains(fake.LastRequest.System, "ALWAYS respond in English") {
					t.Errorf("lang=%q: system prompt không nên bị ghi đè sang tiếng Anh: %q", tt.lang, fake.LastRequest.System)
				}
				return
			}
			if !strings.Contains(fake.LastRequest.System, tt.want) {
				t.Errorf("lang=%q: system prompt = %q, thiếu %q", tt.lang, fake.LastRequest.System, tt.want)
			}
		})
	}
}

// --- helpers cho test ---

var nilEmit EmitFunc = func(Event) {}

// fakeToolMemoryStore implements the unexported tools.memoryBackend interface
// (Set/Search/All) qua structural typing — chỉ cần để tools.NewSaveMemoryTool/
// NewRecallMemoryTool có tham số hợp lệ trong các test ở đây, vốn chỉ quan
// tâm tool có mặt trong FilterToolDefs, không quan tâm dữ liệu bên trong.
type fakeToolMemoryStore struct{}

func newFakeToolMemoryStore() *fakeToolMemoryStore { return &fakeToolMemoryStore{} }

func (*fakeToolMemoryStore) Set(tenantID, key, value string)                 {}
func (*fakeToolMemoryStore) Search(tenantID, query string) map[string]string { return nil }
func (*fakeToolMemoryStore) All(tenantID string) map[string]string           { return nil }

func collectTexts(events []Event) []string {
	var texts []string
	for _, e := range events {
		if e.Type == "text" {
			texts = append(texts, e.Text)
		}
	}
	return texts
}
