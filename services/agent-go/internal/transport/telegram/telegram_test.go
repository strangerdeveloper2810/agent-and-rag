package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/agent"
	"github.com/ai-agent-tut/agent-go/internal/middleware"
	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// fakeRunner implements agent.Runner — giả lập Engine/Orchestrator thật để
// test Bot mà không cần LLM thật, cùng cách cmd/jarvis/main_test.go dùng
// stubRunner.
type fakeRunner struct {
	mu    sync.Mutex
	reply string
	err   error

	calls   int
	lastIn  agent.RunInput
	lastCtx context.Context
}

func (f *fakeRunner) Run(ctx context.Context, in agent.RunInput, emit agent.EmitFunc) (provider.Usage, error) {
	f.mu.Lock()
	f.calls++
	f.lastIn = in
	f.lastCtx = ctx
	f.mu.Unlock()

	if f.err != nil {
		return provider.Usage{}, f.err
	}
	if f.reply != "" {
		emit(agent.TextEvent(f.reply))
	}
	return provider.Usage{}, nil
}

func (f *fakeRunner) snapshot() (calls int, in agent.RunInput, ctx context.Context) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls, f.lastIn, f.lastCtx
}

// fakeTelegramServer giả lập Telegram Bot API: lần getUpdates ĐẦU TIÊN trả
// về đúng updates cấu hình sẵn; các lần sau trả rỗng (như long-poll hết hạn
// không có gì mới). sendMessage được capture lại để assert.
type fakeTelegramServer struct {
	*httptest.Server

	mu           sync.Mutex
	getUpdatesN  int
	firstUpdates []tgUpdate

	sentTexts   []string
	sentChatIDs []int64
}

func newFakeTelegramServer(t *testing.T, token string, firstUpdates []tgUpdate) *fakeTelegramServer {
	t.Helper()
	fs := &fakeTelegramServer{firstUpdates: firstUpdates}

	mux := http.NewServeMux()
	mux.HandleFunc("/bot"+token+"/getUpdates", func(w http.ResponseWriter, r *http.Request) {
		fs.mu.Lock()
		fs.getUpdatesN++
		n := fs.getUpdatesN
		fs.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			_ = json.NewEncoder(w).Encode(tgGetUpdatesResponse{OK: true, Result: fs.firstUpdates})
			return
		}
		// Mô phỏng long-poll hết hạn không có update mới — sleep ngắn để
		// vòng lặp Run() không busy-spin trong lúc test chờ cancel context.
		time.Sleep(10 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(tgGetUpdatesResponse{OK: true, Result: nil})
	})
	mux.HandleFunc("/bot"+token+"/sendMessage", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ChatID int64  `json:"chat_id"`
			Text   string `json:"text"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)

		fs.mu.Lock()
		fs.sentTexts = append(fs.sentTexts, body.Text)
		fs.sentChatIDs = append(fs.sentChatIDs, body.ChatID)
		fs.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tgSendMessageResponse{OK: true})
	})

	fs.Server = httptest.NewServer(mux)
	t.Cleanup(fs.Close)
	return fs
}

func (fs *fakeTelegramServer) sentCount() int {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return len(fs.sentTexts)
}

func (fs *fakeTelegramServer) snapshot() (texts []string, chatIDs []int64) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return append([]string(nil), fs.sentTexts...), append([]int64(nil), fs.sentChatIDs...)
}

// waitFor chờ cond() trả true hoặc hết timeout, poll mỗi 5ms — tránh sleep cố
// định làm test chậm/không ổn định.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("waitFor: điều kiện không đạt sau %s", timeout)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// runBotUntil chạy bot.Run trong goroutine, chờ cond() đạt (hoặc timeout),
// rồi huỷ context và chờ Run trả về — dùng chung cho các test bên dưới.
func runBotUntil(t *testing.T, bot *Bot, runner agent.Runner, timeout time.Duration, cond func() bool) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- bot.Run(ctx, runner) }()

	waitFor(t, timeout, cond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("bot.Run không dừng sau khi huỷ context")
	}
}

func TestBot_ReceivesMessage_RunsAgent_SendsReply(t *testing.T) {
	const token = "123:ABC"
	srv := newFakeTelegramServer(t, token, []tgUpdate{
		{UpdateID: 1, Message: &tgMessage{MessageID: 1, Chat: tgChat{ID: 42}, Text: "xin chao"}},
	})

	bot := New(token)
	bot.apiBase = srv.URL
	runner := &fakeRunner{reply: "chao ban"}

	runBotUntil(t, bot, runner, time.Second, func() bool { return srv.sentCount() >= 1 })

	texts, chatIDs := srv.snapshot()
	if len(texts) != 1 || texts[0] != "chao ban" {
		t.Fatalf("sentTexts = %v, want [\"chao ban\"]", texts)
	}
	if len(chatIDs) != 1 || chatIDs[0] != 42 {
		t.Fatalf("sentChatIDs = %v, want [42]", chatIDs)
	}

	calls, in, _ := runner.snapshot()
	if calls != 1 {
		t.Fatalf("runner.calls = %d, want 1", calls)
	}
	if in.UserMessage != "xin chao" {
		t.Errorf("UserMessage = %q, want %q", in.UserMessage, "xin chao")
	}
}

// TestBot_SetsTenantIDFromChatID kiểm tra tenant ID được set đúng dạng
// "telegram:<chatID>" trong context truyền cho agent.Runner — đây là cơ chế
// cô lập multi-tenant tự động cho user Telegram (mặc định không nằm trong
// OWNER_TENANT_IDS nên tool đặc quyền tự bị chặn).
func TestBot_SetsTenantIDFromChatID(t *testing.T) {
	const token = "123:ABC"
	srv := newFakeTelegramServer(t, token, []tgUpdate{
		{UpdateID: 1, Message: &tgMessage{MessageID: 1, Chat: tgChat{ID: 999}, Text: "hello"}},
	})

	bot := New(token)
	bot.apiBase = srv.URL
	runner := &fakeRunner{reply: "ok"}

	runBotUntil(t, bot, runner, time.Second, func() bool { return srv.sentCount() >= 1 })

	_, _, ctx := runner.snapshot()
	if ctx == nil {
		t.Fatal("runner không nhận được context")
	}
	want := "telegram:999"
	if got := middleware.GetTenantID(ctx); got != want {
		t.Errorf("tenant ID = %q, want %q", got, want)
	}
}

// TestBot_SplitsLongReplyIntoMultipleMessages kiểm tra response dài hơn
// maxMessageLen bị tách thành nhiều lần gọi sendMessage, không có chunk nào
// vượt giới hạn, và ghép lại đúng nội dung gốc.
func TestBot_SplitsLongReplyIntoMultipleMessages(t *testing.T) {
	const token = "123:ABC"
	srv := newFakeTelegramServer(t, token, []tgUpdate{
		{UpdateID: 1, Message: &tgMessage{MessageID: 1, Chat: tgChat{ID: 7}, Text: "gui tra loi dai"}},
	})

	longReply := strings.Repeat("a", 9000) // > 2*4096, cần 3 chunk
	bot := New(token)
	bot.apiBase = srv.URL
	runner := &fakeRunner{reply: longReply}

	runBotUntil(t, bot, runner, time.Second, func() bool { return srv.sentCount() >= 3 })

	texts, chatIDs := srv.snapshot()
	if len(texts) != 3 {
		t.Fatalf("số message gửi = %d, want 3", len(texts))
	}
	var joined strings.Builder
	for _, part := range texts {
		if n := len([]rune(part)); n > maxMessageLen {
			t.Errorf("chunk dài %d ký tự, vượt giới hạn %d", n, maxMessageLen)
		}
		joined.WriteString(part)
		if chatIDs[0] != 7 {
			t.Errorf("chatID sai: %v", chatIDs)
		}
	}
	if joined.String() != longReply {
		t.Error("nối các chunk lại không khớp nội dung gốc")
	}
}

func TestSplitMessage_ShortTextNotSplit(t *testing.T) {
	parts := splitMessage("xin chao", maxMessageLen)
	if len(parts) != 1 || parts[0] != "xin chao" {
		t.Errorf("parts = %v, want 1 phần nguyên vẹn", parts)
	}
}

func TestSplitMessage_ChunksLongText(t *testing.T) {
	long := strings.Repeat("b", 9000)
	parts := splitMessage(long, maxMessageLen)
	if len(parts) != 3 {
		t.Fatalf("len(parts) = %d, want 3", len(parts))
	}

	total := 0
	for i, p := range parts {
		n := len([]rune(p))
		if n > maxMessageLen {
			t.Errorf("part %d dài %d, vượt giới hạn %d", i, n, maxMessageLen)
		}
		total += n
	}
	if total != 9000 {
		t.Errorf("tổng độ dài = %d, want 9000", total)
	}
}

func TestSplitMessage_ExactLimitNotSplit(t *testing.T) {
	s := strings.Repeat("c", maxMessageLen)
	parts := splitMessage(s, maxMessageLen)
	if len(parts) != 1 {
		t.Errorf("len(parts) = %d, want 1 (đúng bằng giới hạn không nên tách)", len(parts))
	}
}

// TestBot_AgentErrorSendsFallbackMessage kiểm tra khi Runner lỗi, Bot gửi 1
// thông báo lỗi thân thiện thay vì im lặng hoặc crash.
func TestBot_AgentErrorSendsFallbackMessage(t *testing.T) {
	const token = "123:ABC"
	srv := newFakeTelegramServer(t, token, []tgUpdate{
		{UpdateID: 1, Message: &tgMessage{MessageID: 1, Chat: tgChat{ID: 5}, Text: "gay loi di"}},
	})

	bot := New(token)
	bot.apiBase = srv.URL
	runner := &fakeRunner{err: fmt.Errorf("boom")}

	runBotUntil(t, bot, runner, time.Second, func() bool { return srv.sentCount() >= 1 })

	texts, _ := srv.snapshot()
	if len(texts) != 1 || texts[0] == "" {
		t.Fatalf("sentTexts = %v, want 1 thông báo lỗi không rỗng", texts)
	}
}

// TestBot_GetUpdatesErrorBacksOffAndRecovers kiểm tra khi getUpdates lỗi
// (500), Bot không crash, chỉ backoff rồi thử lại — vòng lặp Run() vẫn tiếp
// tục hoạt động bình thường sau đó.
func TestBot_GetUpdatesErrorBacksOffAndRecovers(t *testing.T) {
	const token = "123:ABC"

	var mu sync.Mutex
	callN := 0
	var sentTexts []string

	mux := http.NewServeMux()
	mux.HandleFunc("/bot"+token+"/getUpdates", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callN++
		n := callN
		mu.Unlock()

		if n == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("boom"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if n == 2 {
			_ = json.NewEncoder(w).Encode(tgGetUpdatesResponse{OK: true, Result: []tgUpdate{
				{UpdateID: 1, Message: &tgMessage{MessageID: 1, Chat: tgChat{ID: 1}, Text: "hi"}},
			}})
			return
		}
		time.Sleep(10 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(tgGetUpdatesResponse{OK: true, Result: nil})
	})
	mux.HandleFunc("/bot"+token+"/sendMessage", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Text string `json:"text"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		sentTexts = append(sentTexts, body.Text)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tgSendMessageResponse{OK: true})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	bot := New(token)
	bot.apiBase = srv.URL
	runner := &fakeRunner{reply: "van song"}

	runBotUntil(t, bot, runner, 3*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(sentTexts) >= 1
	})

	mu.Lock()
	defer mu.Unlock()
	if len(sentTexts) != 1 || sentTexts[0] != "van song" {
		t.Fatalf("sentTexts = %v, want [\"van song\"] sau khi phục hồi từ lỗi", sentTexts)
	}
}
