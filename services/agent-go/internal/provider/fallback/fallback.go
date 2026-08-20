// Package fallback implements automatic provider failover: when the primary
// LLM provider fails (rate limit, timeout, server error), requests are
// transparently retried on the next provider in the chain.
//
// Pattern: Primary → Fallback1 → Fallback2 → ... → error if all fail.
// Recovery: failed providers are retried after a cooldown period.
// Health tracking: counts consecutive failures and cooldown per provider.
package fallback

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// Provider wraps multiple LLM providers with automatic failover.
type Provider struct {
	chain    []namedProvider
	cooldown time.Duration
	mu       sync.RWMutex
}

type namedProvider struct {
	name      string
	prov      provider.Provider
	coolUntil atomic.Int64 // Unix nano — 0 = available
	failures  atomic.Int64 // consecutive failures
}

// New creates a fallback provider chain. Providers are tried in order.
// cooldown: how long to wait before retrying a failed provider (0 = no cooldown, immediate retry).
func New(cooldown time.Duration, providers ...provider.Provider) (*Provider, error) {
	if len(providers) < 2 {
		return nil, fmt.Errorf("fallback: need at least 2 providers, got %d", len(providers))
	}
	if cooldown < 0 {
		cooldown = 30 * time.Second
	}
	chain := make([]namedProvider, len(providers))
	for i, p := range providers {
		chain[i] = namedProvider{name: p.Name(), prov: p}
	}
	return &Provider{chain: chain, cooldown: cooldown}, nil
}

// Name returns combined names of all providers in the chain.
func (p *Provider) Name() string {
	names := make([]string, len(p.chain))
	for i := range p.chain {
		names[i] = p.chain[i].name
	}
	return "fallback[" + strings.Join(names, "→") + "]"
}

// Generate tries each provider in order. If one fails with a retryable error,
// the next provider is tried. Non-retryable errors (context cancel, invalid args)
// are returned immediately without failover.
func (p *Provider) Generate(ctx context.Context, req provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
	var lastErr error

chainLoop:
	for i := range p.chain {
		np := &p.chain[i]

		// Skip if cooling down
		if coolUntil := np.coolUntil.Load(); coolUntil > 0 {
			if time.Now().UnixNano() < coolUntil {
				continue
			}
			np.coolUntil.Store(0) // cooldown expired
		}

		scoped := scopeModel(req, np.name)

		stream, err := np.prov.Generate(ctx, scoped)
		if err != nil {
			if !isRetryable(err) {
				return nil, err
			}
			lastErr = err
			p.recordFailure(np, err)
			p.logFailure(np, scoped, i, err, "generate")
			continue
		}

		// Đọc tới khi biết chắc CÓ nội dung thật (text/tool call) hoặc gặp lỗi
		// cần forward nguyên trạng cho caller — commit ngay khi biết, không
		// đợi hết stream (đường bình thường không tốn thêm latency).
		//
		// Nếu stream đóng mà HOÀN TOÀN rỗng (không text, không tool call,
		// không lỗi, không bị cắt vì max_tokens) → coi là lỗi retryable NGAY
		// TẠI ĐÂY, thử provider kế tiếp, thay vì forward 1 câu trả lời rỗng
		// cho caller. Trước đây case này (Gemini có lúc trả 200 kèm content
		// rỗng khi gặp lỗi nội bộ, không báo qua ChunkError — xem
		// node_model.go) chỉ bị phát hiện ở tầng dispatch phía TRÊN fallback,
		// quá muộn để thử provider khác — nguyên nhân của log prod
		// "model: empty response" mà KHÔNG có dòng "provider lỗi, thử
		// provider kế tiếp" đứng trước nó.
		var buffered []provider.StreamChunk
		finish := provider.FinishReason("")
		committed := false

		for chunk := range stream {
			buffered = append(buffered, chunk)

			if chunk.Kind == provider.ChunkError {
				if len(buffered) == 1 && isRetryable(chunk.Err) {
					for range stream {
					} // drain phần còn lại trước khi thử provider kế tiếp
					lastErr = chunk.Err
					p.recordFailure(np, chunk.Err)
					p.logFailure(np, scoped, i, chunk.Err, "stream")
					continue chainLoop
				}
				committed = true
				break
			}

			if (chunk.Kind == provider.ChunkText && chunk.Text != "") ||
				chunk.Kind == provider.ChunkToolCall {
				committed = true
				break
			}

			if chunk.Kind == provider.ChunkDone {
				finish = chunk.FinishReason
			}
		}

		if !committed && finish != provider.FinishLength {
			emptyErr := fmt.Errorf("empty response (no text, no tool calls, finish=%q)", finish)
			lastErr = emptyErr
			p.recordFailure(np, emptyErr)
			p.logFailure(np, scoped, i, emptyErr, "empty")
			continue
		}

		// Success — có nội dung thật, hoặc lỗi non-retryable cần forward
		// nguyên trạng cho caller quyết định (giữ nguyên hành vi cũ), hoặc bị
		// cắt vì max_tokens ngay từ đầu (hiếm — node_model.go tự set
		// Truncated dựa trên finish reason).
		np.failures.Store(0)
		// Chỉ log khi đã phải bỏ qua provider nào đó: lượt gọi thành công ngay ở
		// provider đầu là đường bình thường, log nữa chỉ làm nhiễu.
		if i > 0 {
			slog.Info("fallback: provider phục vụ sau khi bỏ qua provider trước",
				"provider", np.name,
				"model", modelOf(scoped, np),
				"chain_index", i,
				"skipped", i,
			)
		}
		return replayBuffered(buffered, stream), nil
	}

	return nil, fmt.Errorf("fallback: all %d providers failed: %w", len(p.chain), lastErr)
}

// logFailure ghi lại việc một provider bị bỏ qua.
//
// Vì sao cần: trước đây package này KHÔNG log gì cả — `recordFailure` chỉ đếm và
// đặt cooldown. Log production vì thế chỉ thấy một loạt dòng "gemini: calling API"
// rồi im lặng, không biết provider nào lỗi, lỗi gì, và cuối cùng ai trả lời. Phân
// tích sự cố phải suy từ mốc thời gian, tức là đoán.
//
// Dùng WARN (không phải ERROR) vì chuỗi fallback vẫn đang làm đúng việc của nó;
// chỉ khi TẤT CẢ provider fail thì Generate mới trả lỗi ra ngoài.
func (p *Provider) logFailure(np *namedProvider, req provider.GenerateRequest, idx int, err error, phase string) {
	slog.Warn("fallback: provider lỗi, thử provider kế tiếp",
		"provider", np.name,
		"model", modelOf(req, np),
		"chain_index", idx,
		"phase", phase, // "generate" = lỗi ngay khi gọi; "stream" = lỗi ở chunk đầu
		"consecutive_failures", np.failures.Load(),
		"daily_quota_exhausted", isDailyQuotaExhausted(err),
		"err", err,
	)
}

// modelNamer là interface TUỲ CHỌN cho provider tự khai tên model nó đang dùng.
//
// Cần vì chuỗi fallback có tới 8 client cùng tên "gemini", mỗi client cấu hình một
// model khác nhau. Nếu chỉ log tên provider thì dòng log "gemini lỗi 429" không
// cho biết model NÀO hết quota — mà đó chính là thông tin để sửa cấu hình.
// Provider không implement thì log rơi về "<provider>:default", không vỡ gì.
type modelNamer interface {
	Model() string
}

// modelOf trả tên model THẬT SỰ dùng cho lượt gọi này, theo thứ tự ưu tiên:
// override trong request → model mặc định do provider tự khai → không biết.
func modelOf(req provider.GenerateRequest, np *namedProvider) string {
	if req.Options.Model != "" {
		return req.Options.Model
	}
	if mn, ok := np.prov.(modelNamer); ok {
		if m := mn.Model(); m != "" {
			return m
		}
	}
	return np.name + ":default"
}

// modelFamily suy ra provider nào sở hữu một tên model. Trả "" khi không nhận ra
// (khi đó ta để nguyên override, vì có thể là model tự host / tên tuỳ biến).
func modelFamily(model string) string {
	m := strings.ToLower(strings.TrimPrefix(model, "models/"))
	switch {
	case m == "":
		return ""
	case strings.HasPrefix(m, "gemini"), strings.HasPrefix(m, "gemma"):
		return "gemini"
	case strings.HasPrefix(m, "deepseek"):
		return "deepseek"
	case strings.HasPrefix(m, "claude"):
		return "anthropic"
	}
	return ""
}

// scopeModel bỏ Options.Model khi tên model KHÔNG thuộc provider đang được gọi.
//
// Lý do: tên model là thứ riêng của từng provider, nhưng cả chuỗi fallback dùng
// chung một GenerateRequest. Caller nào xin model "nhanh/rẻ" (learner,
// SummarizeMessages, trimContext — xem cmd/server/main.go:fastModel) sẽ set
// Options.Model="deepseek-v4-flash"; gemini.Client lại tôn trọng override đó
// (gemini.go: `if req.Options.Model != ""`) nên nó gọi Gemini API với một model
// không tồn tại, chắc chắn lỗi, rồi mới rơi xuống DeepSeek.
//
// Đo trên production: mỗi lượt chat đốt thêm 2 request Gemini vô ích như vậy —
// vừa tốn quota free tier, vừa cộng nhiễu vào circuit breaker/cooldown làm
// provider chính bị đánh dấu "hỏng" oan.
//
// Bỏ override cho provider khác họ = provider đó dùng model mặc định của chính
// nó (cũng là model rẻ), đúng ý "dùng model nhanh nhất có sẵn" mà không tốn
// request rác.
func scopeModel(req provider.GenerateRequest, providerName string) provider.GenerateRequest {
	fam := modelFamily(req.Options.Model)
	if fam == "" || fam == providerName {
		return req
	}
	scoped := req
	scoped.Options.Model = ""
	return scoped
}

func (p *Provider) recordFailure(np *namedProvider, err error) {
	fails := np.failures.Add(1)
	if p.cooldown <= 0 {
		return
	}
	cd := p.cooldown * (1 << min(int(fails)-1, 4))
	if isDailyQuotaExhausted(err) {
		// Day-lock: 2 hours cooldown when daily quota is exhausted
		cd = 2 * time.Hour
	} else if cd > 5*time.Minute {
		cd = 5 * time.Minute
	}
	np.coolUntil.Store(time.Now().Add(cd).UnixNano())
}

// Status returns health status of all providers.
func (p *Provider) Status() []ProviderStatus {
	out := make([]ProviderStatus, len(p.chain))
	for i := range p.chain {
		cu := p.chain[i].coolUntil.Load()
		out[i] = ProviderStatus{
			Name:        p.chain[i].name,
			Failures:    p.chain[i].failures.Load(),
			CoolingDown: cu > 0 && time.Now().UnixNano() < cu,
		}
	}
	return out
}

// replayBuffered trả về channel phát lại các chunk đã đọc để xác định verdict
// (buffered) rồi tiếp tục forward trực tiếp phần CÒN LẠI của stream gốc.
// Timeout 5s mỗi lần gửi: phòng caller bỏ đọc channel (goroutine không rò rỉ
// vô thời hạn).
func replayBuffered(buffered []provider.StreamChunk, stream <-chan provider.StreamChunk) <-chan provider.StreamChunk {
	out := make(chan provider.StreamChunk, 16)
	go func() {
		defer close(out)
		for _, c := range buffered {
			select {
			case out <- c:
			case <-time.After(5 * time.Second):
				return
			}
		}
		for c := range stream {
			select {
			case out <- c:
			case <-time.After(5 * time.Second):
				return
			}
		}
	}()
	return out
}

// ProviderStatus reports health of a single provider.
type ProviderStatus struct {
	Name        string `json:"name"`
	Failures    int64  `json:"failures"`
	CoolingDown bool   `json:"coolingDown"`
}

// --- Helpers ---

func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())

	// Rate limit / server errors → retry
	retryable := []string{
		"429", "rate limit", "too many requests",
		"resource_exhausted", "quota exceeded",
		"503", "502", "500", "internal server error",
		"timeout", "deadline exceeded", "connection refused",
		"temporarily unavailable", "service unavailable",
	}
	for _, p := range retryable {
		if strings.Contains(msg, p) {
			return true
		}
	}

	// Non-retryable → don't failover
	nonRetryable := []string{"400", "401", "403", "invalid", "context canceled"}
	for _, p := range nonRetryable {
		if strings.Contains(msg, p) {
			return false
		}
	}

	// Unknown error → retry (safety: rather retry than fail silently)
	return true
}

func isDailyQuotaExhausted(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	// "perday" (không khoảng trắng): quotaId thật của Gemini là 1 identifier
	// camelCase — "GenerateRequestsPerDayPerProjectPerModel-FreeTier" — lowercase
	// thành "generaterequestsperdayperprojectpermodel..." nên KHÔNG khớp "per day"
	// (có khoảng trắng) hay "free_tier_requests_per_day" (snake_case, sai —
	// metric thật là "generate_content_free_tier_requests", "_per_day" không hề
	// xuất hiện trong đó). Thiếu pattern này khiến production log lúc nào cũng
	// ghi daily_quota_exhausted=false dù lỗi RÕ RÀNG là cạn quota theo ngày —
	// cooldown 2h không bao giờ kích hoạt, hệ thống cứ dò lại provider đã cạn
	// quota cả ngày mỗi vài phút, tốn latency vô ích tới hết ngày.
	return strings.Contains(msg, "per day") ||
		strings.Contains(msg, "perday") ||
		strings.Contains(msg, "daily") ||
		strings.Contains(msg, "day limit") ||
		strings.Contains(msg, "free_tier_requests_per_day") ||
		strings.Contains(msg, "rpd")
}
