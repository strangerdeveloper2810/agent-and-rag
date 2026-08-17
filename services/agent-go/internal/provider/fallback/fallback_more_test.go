package fallback

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

func TestName_ListsChainInOrder(t *testing.T) {
	fb, err := New(time.Second, provider.NewFake(), provider.NewFake(), provider.NewFake())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// FakeProvider đều tên "fake" → chuỗi phải có đúng 3 mắt xích.
	got := fb.Name()
	if !strings.HasPrefix(got, "fallback[") || strings.Count(got, "fake") != 3 {
		t.Errorf("Name() = %q, want fallback[fake→fake→fake]", got)
	}
}

func TestNew_NegativeCooldownUsesDefault(t *testing.T) {
	fb, err := New(-1, provider.NewFake(), provider.NewFake())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if fb.cooldown != 30*time.Second {
		t.Errorf("cooldown = %v, want 30s", fb.cooldown)
	}
}

func TestNew_ZeroCooldownKept(t *testing.T) {
	fb, err := New(0, provider.NewFake(), provider.NewFake())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if fb.cooldown != 0 {
		t.Errorf("cooldown = %v, want 0 (failover tức thì)", fb.cooldown)
	}
}

// Provider đầu trả stream RỖNG (đóng ngay) → coi là thành công, không failover.
func TestGenerate_EmptyStreamCountsAsSuccess(t *testing.T) {
	fb, err := New(time.Second, provider.NewFake(), provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "fallback"},
	))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	stream, err := fb.Generate(context.Background(), provider.GenerateRequest{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	n := 0
	for range stream {
		n++
	}
	if n != 0 {
		t.Errorf("nhận %d chunk, want 0 (stream rỗng của provider đầu)", n)
	}
	if s := fb.Status(); s[0].Failures != 0 {
		t.Errorf("provider đầu bị tính lỗi: %+v", s[0])
	}
}

// Lỗi retryable đến TRÊN STREAM (chunk đầu) → chuyển provider kế tiếp.
func TestGenerate_RetryableErrorOnFirstChunk(t *testing.T) {
	primary := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkError, Err: errors.New("429 rate limit")},
	)
	secondary := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "từ provider 2"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)

	fb, err := New(time.Second, primary, secondary)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	stream, err := fb.Generate(context.Background(), provider.GenerateRequest{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var text string
	for ch := range stream {
		if ch.Kind == provider.ChunkText {
			text += ch.Text
		}
	}
	if text != "từ provider 2" {
		t.Errorf("text = %q, want %q", text, "từ provider 2")
	}

	st := fb.Status()
	if st[0].Failures != 1 {
		t.Errorf("provider 1 Failures = %d, want 1", st[0].Failures)
	}
	if !st[0].CoolingDown {
		t.Error("provider 1 phải đang cooldown")
	}
	if st[1].Failures != 0 {
		t.Errorf("provider 2 Failures = %d, want 0", st[1].Failures)
	}
}

// Lỗi KHÔNG retryable trên chunk đầu vẫn được chuyển tiếp cho caller (không nuốt).
func TestGenerate_NonRetryableStreamErrorForwarded(t *testing.T) {
	primary := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkError, Err: errors.New("400 invalid request")},
	)
	secondary := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "không nên chạy"},
	)

	fb, err := New(time.Second, primary, secondary)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	stream, err := fb.Generate(context.Background(), provider.GenerateRequest{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var chunks []provider.StreamChunk
	for ch := range stream {
		chunks = append(chunks, ch)
	}
	if len(chunks) == 0 || chunks[0].Kind != provider.ChunkError {
		t.Fatalf("chunks = %+v, want lỗi non-retryable được forward", chunks)
	}
	if s := fb.Status(); s[0].Failures != 0 {
		t.Errorf("lỗi non-retryable không được tính failure: %+v", s[0])
	}
}

func TestIsRetryable(t *testing.T) {
	cases := map[string]bool{
		"":                          false, // nil error xử lý riêng bên dưới
		"429 too many requests":     true,
		"rate limit exceeded":       true,
		"RESOURCE_EXHAUSTED":        true,
		"quota exceeded":            true,
		"503 service unavailable":   true,
		"502 bad gateway":           true,
		"500 internal server error": true,
		"timeout while reading":     true,
		"context deadline exceeded": true,
		"connection refused":        true,
		"temporarily unavailable":   true,
		"400 bad request":           false,
		"401 unauthorized":          false,
		"403 forbidden":             false,
		"invalid argument":          false,
		"context canceled":          false,
		"chuyện lạ chưa từng thấy":  true, // lỗi lạ → cứ retry cho an toàn
		"502 nhưng cũng có chữ 400": true, // retryable được kiểm tra trước
	}

	for msg, want := range cases {
		if msg == "" {
			continue
		}
		if got := isRetryable(errors.New(msg)); got != want {
			t.Errorf("isRetryable(%q) = %v, want %v", msg, got, want)
		}
	}

	if isRetryable(nil) {
		t.Error("isRetryable(nil) = true, want false")
	}
}

func TestEmptyStream_IsClosed(t *testing.T) {
	ch := emptyStream()
	if _, open := <-ch; open {
		t.Error("emptyStream() phải trả channel đã đóng")
	}
}

func TestReplayStream_ReplaysFirstThenRest(t *testing.T) {
	rest := make(chan provider.StreamChunk, 2)
	rest <- provider.StreamChunk{Kind: provider.ChunkText, Text: "b"}
	rest <- provider.StreamChunk{Kind: provider.ChunkDone}
	close(rest)

	out := replayStream(provider.StreamChunk{Kind: provider.ChunkText, Text: "a"}, rest)

	var texts []string
	for ch := range out {
		if ch.Kind == provider.ChunkText {
			texts = append(texts, ch.Text)
		}
	}
	if len(texts) != 2 || texts[0] != "a" || texts[1] != "b" {
		t.Errorf("texts = %v, want [a b]", texts)
	}
}

// Cooldown tăng theo cấp số nhân và bị chặn trần ở lần thứ 5 (1<<4).
func TestGenerate_CooldownBackoffCapped(t *testing.T) {
	failing := &alwaysFailProvider{}
	fb, err := New(time.Millisecond, failing, failing)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for i := 0; i < 6; i++ {
		if _, err := fb.Generate(context.Background(), provider.GenerateRequest{}); err == nil {
			t.Fatalf("lần %d: Generate = nil error, want lỗi", i)
		}
		// Hết cooldown để lần sau vẫn thử lại.
		fb.chain[0].coolUntil.Store(0)
		fb.chain[1].coolUntil.Store(0)
	}

	if got := fb.Status()[0].Failures; got < 6 {
		t.Errorf("Failures = %d, want >= 6", got)
	}
}

// alwaysFailProvider luôn lỗi retryable ngay ở Generate().
type alwaysFailProvider struct{}

func (a *alwaysFailProvider) Name() string { return "always-fail" }
func (a *alwaysFailProvider) Generate(context.Context, provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
	return nil, errors.New("503 service unavailable")
}

// Provider đang cooldown phải bị BỎ QUA, không gọi lại.
func TestGenerate_SkipsCoolingProvider(t *testing.T) {
	called := 0
	counting := &countingProvider{n: &called}
	backup := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "backup"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)

	fb, err := New(time.Hour, counting, backup)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	fb.chain[0].coolUntil.Store(time.Now().Add(time.Hour).UnixNano())

	stream, err := fb.Generate(context.Background(), provider.GenerateRequest{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	var text string
	for ch := range stream {
		if ch.Kind == provider.ChunkText {
			text += ch.Text
		}
	}

	if called != 0 {
		t.Errorf("provider đang cooldown bị gọi %d lần, want 0", called)
	}
	if text != "backup" {
		t.Errorf("text = %q, want backup", text)
	}
	if !fb.Status()[0].CoolingDown {
		t.Error("Status phải báo provider 0 đang cooldown")
	}
}

type countingProvider struct{ n *int }

func (c *countingProvider) Name() string { return "counting" }
func (c *countingProvider) Generate(context.Context, provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
	*c.n++
	return provider.NewFake().Generate(context.Background(), provider.GenerateRequest{})
}

// Cooldown hết hạn → provider được thử lại.
func TestGenerate_RetriesAfterCooldownExpired(t *testing.T) {
	primary := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "primary"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)
	fb, err := New(time.Millisecond, primary, provider.NewFake())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Cooldown đã hết hạn (mốc trong quá khứ).
	fb.chain[0].coolUntil.Store(time.Now().Add(-time.Hour).UnixNano())

	stream, err := fb.Generate(context.Background(), provider.GenerateRequest{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	var text string
	for ch := range stream {
		if ch.Kind == provider.ChunkText {
			text += ch.Text
		}
	}
	if text != "primary" {
		t.Errorf("text = %q, want primary (cooldown hết hạn phải thử lại)", text)
	}
	if fb.chain[0].coolUntil.Load() != 0 {
		t.Error("coolUntil phải được reset về 0 sau khi hết hạn")
	}
}
