package agent

import (
	"context"
	"sync"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/tools"
)

// scriptedProvider trả một KỊCH BẢN chunk KHÁC NHAU cho mỗi lần gọi, khác
// provider.NewFake (phát lại cùng một chuỗi mọi lần). Cần thế để mô phỏng tool
// loop thật: lượt 1 model gọi tool, lượt 2 model trả lời bằng text.
type scriptedProvider struct {
	mu      sync.Mutex
	scripts [][]provider.StreamChunk
	calls   int
}

func (s *scriptedProvider) Name() string { return "scripted" }

func (s *scriptedProvider) Generate(_ context.Context, _ provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	script := s.scripts[len(s.scripts)-1]
	if s.calls < len(s.scripts) {
		script = s.scripts[s.calls]
	}
	s.calls++

	ch := make(chan provider.StreamChunk, len(script)+1)
	for _, c := range script {
		ch <- c
	}
	close(ch)
	return ch, nil
}

// Integration test cấp Engine: chạy trọn vẹn engine.Run() qua recall → model →
// tools → model → extract → end, với provider stream theo ĐÚNG cách Gemini làm
// (usageMetadata cộng dồn ở mọi chunk), rồi kiểm tra con số token mà NGƯỜI DÙNG
// thật sự nhận được ở event `done`.
//
// Test cấp node (usage_accounting_test.go) chỉ chứng minh một lượt gọi tính
// đúng; test này chứng minh cả đường ống — engine cộng qua các bước, Usage trả
// về từ Run(), và TotalTokens trong event done — đều không bị nhân lên.
func TestEngineRun_TokenKhongBiNhanLen_QuaToolLoop(t *testing.T) {
	// Lượt 1: model gọi tool `echo`. Gemini gửi 3 chunk usage, mỗi chunk mang
	// promptTokenCount ĐẦY ĐỦ (1000) — cộng dồn sẽ thành 3000.
	step1 := []provider.StreamChunk{
		{Kind: provider.ChunkUsage, Usage: &provider.Usage{InputTokens: 1000, OutputTokens: 5}},
		{Kind: provider.ChunkToolCall, ToolCall: &provider.ToolCall{
			ID: "call-1", Name: "echo", Args: []byte(`{"text":"xin chao"}`),
		}},
		{Kind: provider.ChunkUsage, Usage: &provider.Usage{InputTokens: 1000, OutputTokens: 12}},
		{Kind: provider.ChunkUsage, Usage: &provider.Usage{InputTokens: 1000, OutputTokens: 20}},
		{Kind: provider.ChunkDone},
	}
	// Lượt 2: model trả lời bằng text. Input lớn hơn vì đã có tool result trong
	// history — đúng như thật.
	step2 := []provider.StreamChunk{
		{Kind: provider.ChunkText, Text: "Đã chạy tool. "},
		{Kind: provider.ChunkUsage, Usage: &provider.Usage{InputTokens: 1500, OutputTokens: 4}},
		{Kind: provider.ChunkText, Text: "Kết quả: xin chao"},
		{Kind: provider.ChunkUsage, Usage: &provider.Usage{InputTokens: 1500, OutputTokens: 30}},
		{Kind: provider.ChunkDone},
	}

	prov := &scriptedProvider{scripts: [][]provider.StreamChunk{step1, step2}}

	reg := tools.NewRegistry()
	reg.Register(tools.NewEchoTool())

	eng := NewEngine(prov, reg)

	var (
		mu       sync.Mutex
		doneEvts []Event
		texts    []string
	)
	emit := func(e Event) {
		mu.Lock()
		defer mu.Unlock()
		switch e.Type {
		case "done":
			doneEvts = append(doneEvts, e)
		case "text":
			texts = append(texts, e.Text)
		}
	}

	usage, err := eng.Run(context.Background(), RunInput{
		UserMessage: "chạy echo giúp tôi",
		MaxSteps:    5,
	}, emit)
	if err != nil {
		t.Fatalf("engine.Run lỗi: %v", err)
	}

	// Hai bước model: input 1000 + 1500 = 2500; output 20 + 30 = 50.
	// Nếu cộng dồn từng chunk usage (bug cũ): input = 3000 + 3000 = 6000.
	const wantIn, wantOut = 2500, 50

	if usage.InputTokens != wantIn {
		t.Errorf("Usage.InputTokens = %d, want %d (cộng dồn chunk sẽ ra 6000)", usage.InputTokens, wantIn)
	}
	if usage.OutputTokens != wantOut {
		t.Errorf("Usage.OutputTokens = %d, want %d", usage.OutputTokens, wantOut)
	}

	if prov.calls != 2 {
		t.Errorf("số lượt gọi provider = %d, want 2 (model → tool → model)", prov.calls)
	}

	// Con số mà client SSE thật sự nhận được.
	if len(doneEvts) != 1 {
		t.Fatalf("số event done = %d, want 1", len(doneEvts))
	}
	if got := doneEvts[0].TotalTokens; got != wantIn+wantOut {
		t.Errorf("done.TotalTokens = %d, want %d", got, wantIn+wantOut)
	}
	if doneEvts[0].Usage == nil || doneEvts[0].Usage.InputTokens != wantIn {
		t.Errorf("done.Usage = %+v, want InputTokens=%d", doneEvts[0].Usage, wantIn)
	}

	// Và câu trả lời vẫn tới được người dùng nguyên vẹn.
	if len(texts) == 0 {
		t.Error("không có event text nào — tool loop không hoàn tất")
	}
}

// Một lượt chat thường (không tool) đi hết engine: kiểm tra không có bước nào
// cộng thêm token ngoài lượt gọi model duy nhất.
func TestEngineRun_MotLuotDuyNhat_TokenDungBangSnapshot(t *testing.T) {
	prov := &scriptedProvider{scripts: [][]provider.StreamChunk{{
		{Kind: provider.ChunkText, Text: "Xin chào!"},
		{Kind: provider.ChunkUsage, Usage: &provider.Usage{InputTokens: 5260, OutputTokens: 3}},
		{Kind: provider.ChunkUsage, Usage: &provider.Usage{InputTokens: 5260, OutputTokens: 9}},
		{Kind: provider.ChunkDone},
	}}}

	eng := NewEngine(prov, tools.NewRegistry())

	usage, err := eng.Run(context.Background(), RunInput{
		UserMessage: "Xin chào",
		MaxSteps:    5,
	}, func(Event) {})
	if err != nil {
		t.Fatalf("engine.Run lỗi: %v", err)
	}

	if usage.InputTokens != 5260 || usage.OutputTokens != 9 {
		t.Errorf("Usage = %+v, want {5260 9}", usage)
	}
}
