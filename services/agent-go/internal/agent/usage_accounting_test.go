package agent

import (
	"context"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/tools"
)

// Gemini gửi usageMetadata kèm promptTokenCount ĐẦY ĐỦ ở MỌI chunk stream, và
// candidatesTokenCount tăng dần — tức mỗi ChunkUsage là một SNAPSHOT cộng dồn,
// không phải delta. Cộng dồn các snapshot đó (bug cũ) làm input token bị nhân
// với số chunk: đo trên production, một request gửi ~5.200 token bị báo 41.400.
//
// Hệ quả không chỉ ở log: totalTokens chảy ra UI và vào contextTokens/
// contextBudget mà FE dùng để gợi ý "nên bắt đầu chat mới".
func TestNodeModel_UsageGemini_SnapshotKhongCongDon(t *testing.T) {
	fake := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "Xin "},
		provider.StreamChunk{Kind: provider.ChunkUsage, Usage: &provider.Usage{InputTokens: 5200, OutputTokens: 2}},
		provider.StreamChunk{Kind: provider.ChunkText, Text: "chào "},
		provider.StreamChunk{Kind: provider.ChunkUsage, Usage: &provider.Usage{InputTokens: 5200, OutputTokens: 5}},
		provider.StreamChunk{Kind: provider.ChunkText, Text: "bạn!"},
		provider.StreamChunk{Kind: provider.ChunkUsage, Usage: &provider.Usage{InputTokens: 5200, OutputTokens: 9}},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)

	eng := &fakeEngine{prov: fake, registry: tools.NewRegistry()}
	s := newState(RunInput{UserMessage: "Xin chào", MaxSteps: 12})

	if _, err := nodeModel(context.Background(), eng, s, func(Event) {}); err != nil {
		t.Fatalf("nodeModel error: %v", err)
	}

	if s.Usage.InputTokens != 5200 {
		t.Errorf("InputTokens = %d, want 5200 (snapshot, không phải 3×5200)", s.Usage.InputTokens)
	}
	if s.Usage.OutputTokens != 9 {
		t.Errorf("OutputTokens = %d, want 9 (snapshot cuối, không phải 2+5+9)", s.Usage.OutputTokens)
	}
	if s.TotalTokens != 5209 {
		t.Errorf("TotalTokens = %d, want 5209", s.TotalTokens)
	}
}

// Anthropic/DeepSeek chỉ gửi usage MỘT lần ở cuối stream — kiểu này vẫn phải
// tính đúng sau khi đổi sang semantics snapshot.
func TestNodeModel_UsageMotLanODuoiCung(t *testing.T) {
	fake := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "ok"},
		provider.StreamChunk{Kind: provider.ChunkUsage, Usage: &provider.Usage{InputTokens: 1234, OutputTokens: 56}},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)

	eng := &fakeEngine{prov: fake, registry: tools.NewRegistry()}
	s := newState(RunInput{UserMessage: "hi", MaxSteps: 12})

	if _, err := nodeModel(context.Background(), eng, s, func(Event) {}); err != nil {
		t.Fatalf("nodeModel error: %v", err)
	}

	if s.Usage.InputTokens != 1234 || s.Usage.OutputTokens != 56 {
		t.Errorf("Usage = %+v, want {1234 56}", s.Usage)
	}
}

// Nhiều BƯỚC (tool loop) thì usage PHẢI cộng dồn qua các bước — snapshot chỉ áp
// dụng trong phạm vi một lượt gọi provider.
func TestNodeModel_UsageCongDonQuaNhieuBuoc(t *testing.T) {
	fake := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "buoc 1"},
		provider.StreamChunk{Kind: provider.ChunkUsage, Usage: &provider.Usage{InputTokens: 100, OutputTokens: 10}},
		provider.StreamChunk{Kind: provider.ChunkUsage, Usage: &provider.Usage{InputTokens: 100, OutputTokens: 20}},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)

	eng := &fakeEngine{prov: fake, registry: tools.NewRegistry()}
	s := newState(RunInput{UserMessage: "hi", MaxSteps: 12})

	// Gọi node model 2 lần như tool loop vẫn làm.
	for i := 0; i < 2; i++ {
		if _, err := nodeModel(context.Background(), eng, s, func(Event) {}); err != nil {
			t.Fatalf("nodeModel lần %d lỗi: %v", i+1, err)
		}
	}

	if s.Usage.InputTokens != 200 {
		t.Errorf("InputTokens = %d, want 200 (100 mỗi bước × 2 bước)", s.Usage.InputTokens)
	}
	if s.Usage.OutputTokens != 40 {
		t.Errorf("OutputTokens = %d, want 40 (20 mỗi bước × 2 bước)", s.Usage.OutputTokens)
	}
}
