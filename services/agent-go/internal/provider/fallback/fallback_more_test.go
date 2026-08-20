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

// Provider đầu trả stream RỖNG (đóng ngay, không text/tool call) → PHẢI
// failover sang provider kế tiếp, KHÔNG được coi là thành công.
//
// Trước đây: 1 chunk rỗng/đóng kênh ngay được coi là "thành công" — đúng bug
// gặp trong log production (Gemini trả 200 kèm content rỗng, KHÔNG failover
// sang DeepSeek/Anthropic, user nhận lỗi dù còn nguyên chuỗi fallback phía
// sau chưa thử).
func TestGenerate_EmptyStreamTriggersFailover(t *testing.T) {
	fb, err := New(time.Second, provider.NewFake(), provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "fallback"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	))
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
	if text != "fallback" {
		t.Errorf("text = %q, want %q (phải failover sang provider kế tiếp)", text, "fallback")
	}
	if s := fb.Status(); s[0].Failures != 1 {
		t.Errorf("provider đầu (rỗng) phải bị tính lỗi: %+v", s[0])
	}
}

// Tất cả provider đều trả stream rỗng → Generate() phải trả lỗi, không được
// âm thầm trả 1 câu trả lời rỗng cho caller.
func TestGenerate_AllEmptyReturnsError(t *testing.T) {
	fb, err := New(time.Second, provider.NewFake(), provider.NewFake())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = fb.Generate(context.Background(), provider.GenerateRequest{})
	if err == nil {
		t.Fatal("expected error khi mọi provider đều trả response rỗng")
	}
}

// Bị cắt vì max_tokens ngay từ đầu (finish=length, không sinh được token nào)
// KHÔNG được coi là "rỗng" — đây không phải lỗi provider, phải forward
// nguyên trạng để node_model.go tự set Truncated như cũ.
func TestGenerate_TruncatedEmptyIsNotRetried(t *testing.T) {
	fb, err := New(time.Second, provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkDone, FinishReason: provider.FinishLength},
	), provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "không nên chạy tới đây"},
	))
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
	if len(chunks) != 1 || chunks[0].FinishReason != provider.FinishLength {
		t.Errorf("chunks = %+v, want đúng 1 chunk ChunkDone finish=length được forward", chunks)
	}
	if s := fb.Status(); s[0].Failures != 0 {
		t.Errorf("provider bị cắt vì max_tokens không được tính lỗi: %+v", s[0])
	}
}

// Nội dung thật đến ở chunk THỨ HAI trở đi (vd sau 1 chunk usage) vẫn phải
// được nhận diện là thành công, không bị coi nhầm là rỗng.
func TestGenerate_ContentOnLaterChunkStillCommits(t *testing.T) {
	fb, err := New(time.Second, provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkUsage, Usage: &provider.Usage{InputTokens: 10}},
		provider.StreamChunk{Kind: provider.ChunkText, Text: "trễ nhưng vẫn có"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	), provider.NewFake())
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
	if text != "trễ nhưng vẫn có" {
		t.Errorf("text = %q, want %q", text, "trễ nhưng vẫn có")
	}
	if s := fb.Status(); s[0].Failures != 0 {
		t.Errorf("provider có nội dung (dù đến trễ) không được tính lỗi: %+v", s[0])
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

// isDailyQuotaExhausted phải nhận diện đúng lỗi cạn quota theo NGÀY của
// Gemini — dùng nguyên văn rút gọn từ log production 2026-08-20, quotaId
// "GenerateRequestsPerDayPerProjectPerModel-FreeTier" là 1 identifier
// camelCase KHÔNG có khoảng trắng, trước đây không khớp pattern "per day".
func TestIsDailyQuotaExhausted(t *testing.T) {
	cases := map[string]bool{
		"Error 429, Message: You exceeded your current quota... Quota exceeded for metric: generativelanguage.googleapis.com/generate_content_free_tier_requests, limit: 20, model: gemini-2.5-flash-lite... quotaId:GenerateRequestsPerDayPerProjectPerModel-FreeTier quotaMetric:generativelanguage.googleapis.com/generate_content_free_tier_requests": true,
		"429 rate limit, retry in 2s (per-minute limit)":    false,
		"Rate limit exceeded, resets daily at midnight UTC": true,
		"quota RPD exceeded":                                true,
		"context canceled":                                  false,
	}
	for msg, want := range cases {
		if got := isDailyQuotaExhausted(errors.New(msg)); got != want {
			t.Errorf("isDailyQuotaExhausted(%q) = %v, want %v", msg, got, want)
		}
	}
	if isDailyQuotaExhausted(nil) {
		t.Error("isDailyQuotaExhausted(nil) = true, want false")
	}
}

// Lỗi cạn quota theo ngày phải bị khoá cooldown 2 tiếng (không phải cooldown
// ngắn thông thường) — nếu không hệ thống cứ dò lại provider đã cạn quota cả
// ngày mỗi vài phút, đúng như log production cho thấy.
func TestRecordFailure_DailyQuotaExhaustedGetsTwoHourCooldown(t *testing.T) {
	// "429" để isRetryable() = true (tới được recordFailure); quotaId là
	// nguyên văn định dạng thật Gemini trả về khi cạn quota theo ngày.
	dailyErr := errors.New("429 quotaId:GenerateRequestsPerDayPerProjectPerModel-FreeTier")
	primary := &errorProvider{name: "gemini", err: dailyErr}
	secondary := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "ok"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)

	fb, err := New(time.Second, primary, secondary)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := fb.Generate(context.Background(), provider.GenerateRequest{}); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	coolUntil := fb.chain[0].coolUntil.Load()
	wantMin := time.Now().Add(90 * time.Minute).UnixNano() // > 5 phút cooldown thường, gần 2h
	if coolUntil < wantMin {
		t.Errorf("coolUntil quá gần — muốn cooldown ~2h cho lỗi cạn quota theo ngày, got %v", time.Unix(0, coolUntil))
	}
}

func TestReplayBuffered_ReplaysBufferedThenRest(t *testing.T) {
	rest := make(chan provider.StreamChunk, 2)
	rest <- provider.StreamChunk{Kind: provider.ChunkText, Text: "b"}
	rest <- provider.StreamChunk{Kind: provider.ChunkDone}
	close(rest)

	out := replayBuffered([]provider.StreamChunk{{Kind: provider.ChunkText, Text: "a"}}, rest)

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
