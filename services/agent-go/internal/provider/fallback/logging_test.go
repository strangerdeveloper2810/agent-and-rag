package fallback

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// captureLogs chuyển slog default sang một buffer trong phạm vi test rồi trả về
// hàm đọc nội dung đã log. Level Debug để không bỏ sót gì.
func captureLogs(t *testing.T) func() string {
	t.Helper()

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	return buf.String
}

// Trước đây package này không log gì: log production chỉ thấy một loạt dòng
// "gemini: calling API" rồi im lặng, không biết provider nào lỗi, lỗi gì, và ai
// trả lời cuối cùng — phải suy từ mốc thời gian.
func TestFallback_LogLoiCuaTungProvider(t *testing.T) {
	logs := captureLogs(t)

	broken := &errorProvider{name: "gemini", err: errors.New("429 Too Many Requests")}
	good := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "ok"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)

	fb, err := New(time.Second, broken, good)
	if err != nil {
		t.Fatal(err)
	}

	stream, err := fb.Generate(context.Background(), provider.GenerateRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for range stream {
	}

	out := logs()

	if !strings.Contains(out, "provider lỗi") {
		t.Fatalf("không có log WARN khi provider lỗi:\n%s", out)
	}
	// Phải nêu ĐỦ để chẩn đoán: provider nào, model nào, vị trí trong chain, lỗi gì.
	for _, want := range []string{
		`provider=gemini`,
		`chain_index=0`,
		`phase=generate`,
		`429 Too Many Requests`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("log thiếu %q:\n%s", want, out)
		}
	}
}

// Biết ai THẬT SỰ trả lời cũng quan trọng như biết ai lỗi: nếu câu trả lời đến từ
// provider thứ 5 trong chain thì chi phí và chất lượng khác hẳn provider đầu.
func TestFallback_LogProviderCuoiCungPhucVu(t *testing.T) {
	logs := captureLogs(t)

	broken := &errorProvider{name: "gemini", err: errors.New("503 Service Unavailable")}
	good := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "ok"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)

	fb, _ := New(time.Second, broken, good)
	stream, err := fb.Generate(context.Background(), provider.GenerateRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for range stream {
	}

	out := logs()
	if !strings.Contains(out, "provider phục vụ sau khi bỏ qua") {
		t.Errorf("thiếu log INFO cho provider đã phục vụ:\n%s", out)
	}
	if !strings.Contains(out, "chain_index=1") {
		t.Errorf("log không nói provider thứ mấy đã phục vụ:\n%s", out)
	}
}

// Đường bình thường (provider đầu trả lời ngay) KHÔNG được log gì thêm — mỗi lượt
// chat log một dòng vô nghĩa thì log production thành rác.
func TestFallback_KhongLogKhiProviderDauThanhCong(t *testing.T) {
	logs := captureLogs(t)

	primary := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "ok"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)
	secondary := provider.NewFake(provider.StreamChunk{Kind: provider.ChunkDone})

	fb, _ := New(time.Second, primary, secondary)
	stream, err := fb.Generate(context.Background(), provider.GenerateRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for range stream {
	}

	if out := logs(); strings.Contains(out, "fallback:") {
		t.Errorf("log thừa ở đường bình thường:\n%s", out)
	}
}

// Lỗi đến GIỮA stream (không phải lúc gọi) là ca thật đã thấy trong log dev —
// phải phân biệt được bằng field phase.
func TestFallback_LogPhanBietLoiTrongStream(t *testing.T) {
	logs := captureLogs(t)

	streamErr := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkError, Err: errors.New("429 rate limit")},
	)
	good := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "ok"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)

	fb, _ := New(time.Second, streamErr, good)
	stream, err := fb.Generate(context.Background(), provider.GenerateRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for range stream {
	}

	if out := logs(); !strings.Contains(out, "phase=stream") {
		t.Errorf("lỗi giữa stream phải được log với phase=stream:\n%s", out)
	}
}

type stubModelProvider struct {
	*provider.FakeProvider
	model string
}

func (s *stubModelProvider) Model() string { return s.model }

func TestModelOf(t *testing.T) {
	withModel := provider.GenerateRequest{
		Options: provider.ProviderOptions{Model: "gemini-3.1-flash-lite"},
	}
	npGemini := &namedProvider{name: "gemini", prov: provider.NewFake()}
	if got := modelOf(withModel, npGemini); got != "gemini-3.1-flash-lite" {
		t.Errorf("modelOf = %q, want tên model được yêu cầu", got)
	}

	// Provider có Model() method tự khai
	npWithModel := &namedProvider{name: "gemini", prov: &stubModelProvider{FakeProvider: provider.NewFake(), model: "gemini-2.5-flash"}}
	if got := modelOf(provider.GenerateRequest{}, npWithModel); got != "gemini-2.5-flash" {
		t.Errorf("modelOf = %q, want gemini-2.5-flash", got)
	}

	// Options.Model rỗng & provider không có Model() method -> fallback về provider:default
	npDeepseek := &namedProvider{name: "deepseek", prov: provider.NewFake()}
	if got := modelOf(provider.GenerateRequest{}, npDeepseek); got != "deepseek:default" {
		t.Errorf("modelOf = %q, want %q", got, "deepseek:default")
	}
}
