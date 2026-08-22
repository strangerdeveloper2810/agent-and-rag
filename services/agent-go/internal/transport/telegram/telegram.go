// Package telegram implements a Telegram Bot channel for JARVIS using
// long-polling (getUpdates) — KHÔNG dùng webhook, nên không cần domain/SSL
// công khai, phù hợp chạy trên máy cá nhân/VPS không có ingress public.
//
// Bot tái dùng CHÍNH agent.Runner (Engine hoặc Orchestrator) đã được wire ở
// cmd/server/main.go cho kênh HTTP/SSE — không dựng engine riêng, nên mọi
// cấu hình (tool registry, system prompt, memory, guardrails, owner tenant…)
// áp dụng giống hệt kênh /chat.
//
// Client Telegram Bot API viết THUẦN qua net/http (không thêm dependency SDK
// ngoài), theo đúng văn hoá "stdlib only" của agent-go.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/agent"
	"github.com/ai-agent-tut/agent-go/internal/middleware"
	"github.com/ai-agent-tut/agent-go/internal/provider"
)

const (
	// defaultAPIBase là gốc URL Telegram Bot API thật. Test override qua
	// field apiBase (unexported, cùng package) để trỏ vào httptest.Server giả.
	defaultAPIBase = "https://api.telegram.org"

	// maxMessageLen là giới hạn ký tự của Telegram cho 1 lần sendMessage.
	// Response dài hơn phải tách thành nhiều tin nhắn liên tiếp.
	maxMessageLen = 4096

	// pollTimeoutSec là tham số "timeout" gửi cho getUpdates (long-polling):
	// Telegram giữ request mở tối đa ngần này giây, trả về ngay khi có update
	// mới, hoặc trả rỗng khi hết hạn — tránh polling dồn dập kiểu short-poll.
	pollTimeoutSec = 30

	// httpClientTimeout PHẢI lớn hơn pollTimeoutSec để không cắt ngang chính
	// long-poll request đang chờ Telegram phản hồi.
	httpClientTimeout = 45 * time.Second

	// initialBackoff/maxBackoff: backoff tăng dần khi getUpdates lỗi (rate
	// limit, network) — không crash cả process, chỉ chậm lại rồi thử tiếp.
	initialBackoff = 500 * time.Millisecond
	maxBackoff     = 30 * time.Second

	// maxSendRetries là số lần thử lại tối đa khi sendMessage lỗi cho 1 chunk.
	maxSendRetries = 3
)

// Bot là Telegram long-polling channel cho JARVIS.
type Bot struct {
	token   string
	apiBase string
	client  *http.Client

	// history giữ hội thoại gần đây theo chatID TRONG RAM — mất khi process
	// restart. Đây là giới hạn CÓ CHỦ ĐÍCH của v1: persist xuống SQLite/Mongo
	// là over-engineer khi chưa rõ nhu cầu thật, và mỗi chatID Telegram đã tự
	// nhiên map 1-1 sang 1 tenant ("telegram:<chatID>") nên không có rủi ro
	// rò rỉ hội thoại giữa các user — chỉ là không "nhớ" qua lần khởi động lại.
	historyMu sync.Mutex
	history   map[int64][]provider.Message
}

// New tạo Bot với token thật (BotFather). apiBase mặc định là Telegram thật;
// test trong package này có thể ghi đè field apiBase để trỏ vào server giả.
func New(token string) *Bot {
	return &Bot{
		token:   token,
		apiBase: defaultAPIBase,
		client:  &http.Client{Timeout: httpClientTimeout},
		history: make(map[int64][]provider.Message),
	}
}

// --- Telegram Bot API wire types (chỉ các field JARVIS cần) ---

type tgUpdate struct {
	UpdateID int64      `json:"update_id"`
	Message  *tgMessage `json:"message,omitempty"`
}

type tgMessage struct {
	MessageID int64  `json:"message_id"`
	Chat      tgChat `json:"chat"`
	Text      string `json:"text"`
}

type tgChat struct {
	ID int64 `json:"id"`
}

type tgGetUpdatesResponse struct {
	OK          bool       `json:"ok"`
	Description string     `json:"description,omitempty"`
	Result      []tgUpdate `json:"result"`
}

type tgSendMessageResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description,omitempty"`
}

// Run bắt đầu vòng lặp long-polling: gọi getUpdates lặp lại, xử lý từng
// message mới qua orch, gửi trả lời qua sendMessage. Chặn (blocking) cho tới
// khi ctx bị huỷ — dùng để chạy trong goroutine riêng (xem cmd/server/main.go).
//
// orch nhận kiểu agent.Runner (interface, không phải *orchestrator.Orchestrator
// cụ thể) để test được bằng runner giả, và để Bot hoạt động y hệt dù caller
// truyền 1 Engine đơn lẻ hay 1 Orchestrator đa agent.
func (b *Bot) Run(ctx context.Context, orch agent.Runner) error {
	var offset int64
	backoff := initialBackoff

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		updates, err := b.getUpdates(ctx, offset)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			slog.Error("telegram: getUpdates lỗi — thử lại sau backoff", "err", err, "backoff", backoff)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}
		backoff = initialBackoff // reset sau khi thành công

		for _, u := range updates {
			offset = u.UpdateID + 1
			if u.Message == nil || strings.TrimSpace(u.Message.Text) == "" {
				continue // bỏ qua update không phải text message (ảnh, sticker, ...)
			}
			b.handleMessage(ctx, orch, u.Message.Chat.ID, u.Message.Text)
		}
	}
}

// handleMessage chạy 1 lượt agent cho message của chatID, cập nhật history
// trong RAM, rồi gửi trả lời (tách chunk nếu dài quá maxMessageLen).
func (b *Bot) handleMessage(ctx context.Context, orch agent.Runner, chatID int64, text string) {
	// "telegram:<chatID>" làm tenant ID → tự động cô lập theo cơ chế
	// multi-tenant đã có (middleware.GetTenantID đọc lại giá trị này trong
	// mọi tool/memory). Mặc định KHÔNG nằm trong OWNER_TENANT_IDS nên tool
	// đặc quyền (shell.exec, file.*) tự động bị chặn cho user Telegram lạ —
	// đây là hành vi ĐÚNG và AN TOÀN (xem tools.IsOwnerTenant), không cần
	// thêm code chặn riêng ở đây.
	tenantID := fmt.Sprintf("telegram:%d", chatID)
	tenantCtx := context.WithValue(ctx, middleware.TenantIDKey, tenantID)

	b.historyMu.Lock()
	history := append([]provider.Message(nil), b.history[chatID]...)
	b.historyMu.Unlock()

	input := agent.RunInput{
		ConversationID: tenantID,
		History:        history,
		UserMessage:    text,
		MaxSteps:       12,
		Lang:           "vi",
	}

	var reply strings.Builder
	emit := func(e agent.Event) {
		// v1 không streaming từng chữ về Telegram (API sendMessage không hỗ
		// trợ edit-liên-tục hợp lý cho long-polling đơn giản) — gom hết event
		// "text" rồi gửi 1 lần (hoặc nhiều lần nếu dài) khi lượt chạy xong.
		if e.Type == "text" {
			reply.WriteString(e.Text)
		}
	}

	if _, err := orch.Run(tenantCtx, input, emit); err != nil {
		slog.Error("telegram: agent run lỗi", "chat_id", chatID, "err", err)
		b.sendReply(ctx, chatID, "Xin lỗi, đã có lỗi xảy ra khi xử lý yêu cầu của bạn.")
		return
	}

	respText := reply.String()
	if respText == "" {
		return
	}

	b.historyMu.Lock()
	b.history[chatID] = append(b.history[chatID],
		provider.Message{Role: provider.RoleUser, Content: text},
		provider.Message{Role: provider.RoleAssistant, Content: respText},
	)
	b.historyMu.Unlock()

	b.sendReply(ctx, chatID, respText)
}

// sendReply gửi text tới chatID, tách thành nhiều tin nhắn nếu vượt
// maxMessageLen (giới hạn Telegram). Lỗi gửi từng chunk chỉ log, không panic —
// một chunk lỗi không nên chặn các chunk còn lại hay làm sập vòng lặp chính.
func (b *Bot) sendReply(ctx context.Context, chatID int64, text string) {
	for _, chunk := range splitMessage(text, maxMessageLen) {
		if err := b.sendMessageWithRetry(ctx, chatID, chunk); err != nil {
			slog.Error("telegram: gửi tin nhắn thất bại sau khi retry", "chat_id", chatID, "err", err)
		}
	}
}

// splitMessage tách s thành các đoạn tối đa limit ký tự (rune, không phải
// byte — text tiếng Việt multi-byte sẽ bị cắt sai vị trí nếu đếm theo byte).
func splitMessage(s string, limit int) []string {
	if limit <= 0 {
		limit = maxMessageLen
	}
	runes := []rune(s)
	if len(runes) <= limit {
		return []string{s}
	}

	parts := make([]string, 0, len(runes)/limit+1)
	for len(runes) > 0 {
		n := limit
		if n > len(runes) {
			n = len(runes)
		}
		parts = append(parts, string(runes[:n]))
		runes = runes[n:]
	}
	return parts
}

// getUpdates gọi Telegram Bot API getUpdates với offset/timeout long-polling.
func (b *Bot) getUpdates(ctx context.Context, offset int64) ([]tgUpdate, error) {
	url := fmt.Sprintf("%s/bot%s/getUpdates?offset=%d&timeout=%d", b.apiBase, b.token, offset, pollTimeoutSec)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("telegram: build getUpdates request: %w", err)
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("telegram: getUpdates request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("telegram: getUpdates status %d: %s", resp.StatusCode, string(body))
	}

	var out tgGetUpdatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("telegram: decode getUpdates response: %w", err)
	}
	if !out.OK {
		return nil, fmt.Errorf("telegram: getUpdates not ok: %s", out.Description)
	}
	return out.Result, nil
}

// sendMessage gọi Telegram Bot API sendMessage 1 lần (không retry — retry ở
// sendMessageWithRetry để tách rõ concern network I/O vs. policy chịu lỗi).
func (b *Bot) sendMessage(ctx context.Context, chatID int64, text string) error {
	payload, err := json.Marshal(map[string]any{
		"chat_id": chatID,
		"text":    text,
	})
	if err != nil {
		return fmt.Errorf("telegram: encode sendMessage payload: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/sendMessage", b.apiBase, b.token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("telegram: build sendMessage request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: sendMessage request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram: sendMessage status %d: %s", resp.StatusCode, string(body))
	}

	var out tgSendMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fmt.Errorf("telegram: decode sendMessage response: %w", err)
	}
	if !out.OK {
		return fmt.Errorf("telegram: sendMessage not ok: %s", out.Description)
	}
	return nil
}

// sendMessageWithRetry thử sendMessage tối đa maxSendRetries lần với backoff
// tăng dần — cùng tinh thần với backoff của getUpdates trong Run: lỗi mạng/
// rate limit tạm thời không nên làm mất hẳn 1 câu trả lời.
func (b *Bot) sendMessageWithRetry(ctx context.Context, chatID int64, text string) error {
	backoff := initialBackoff
	var lastErr error
	for attempt := 1; attempt <= maxSendRetries; attempt++ {
		if err := b.sendMessage(ctx, chatID, text); err != nil {
			lastErr = err
			if attempt == maxSendRetries {
				break
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}
		return nil
	}
	return lastErr
}
