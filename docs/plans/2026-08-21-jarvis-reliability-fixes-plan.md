# JARVIS agent-go Reliability Fixes Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Sửa 3 lỗi tìm thấy qua log production (routing tự lạc đề, reflection đốt quota Gemini, model hallucination không grounding) và thêm 1 tính năng resilience (Claude API key thứ 2 làm fallback) cho `services/agent-go`.

**Architecture:** 7 task độc lập, mỗi task sửa 1-3 file Go trong `services/agent-go`, theo TDD (test trước, code sau). Không đổi kiến trúc engine/orchestrator tổng thể — chỉ thêm state nhỏ (sticky-agent map, turn counter) và 1 rule mới trong system prompt. Deploy tuần tự theo nhóm: Routing (Task 1-2) → Quota (Task 3-5) → Claude 2-key (Task 6) → Hallucination (Task 7), verify qua log VPS giữa mỗi nhóm.

**Tech Stack:** Go 1.25+, `services/agent-go` monorepo package layout (`internal/orchestrator`, `internal/memory`, `internal/provider/factory`, `internal/config`, `internal/agent`).

**Design doc:** `docs/plans/2026-08-21-jarvis-reliability-fixes-design.md`

---

## Ghi chú an toàn chung

- Chạy `cd services/agent-go && go build ./... && go test ./...` sau MỖI task trước khi commit.
- `services/agent-go` không hot reload — deploy thật (sau khi nhóm task xong) qua `./deploy/deploy-to-vps.sh` từ root repo, KHÔNG chạy trên VPS.
- Verify sau deploy: `ssh hr-vps "docker logs --tail 200 --timestamps jarvis-agent-go"`.
- Mọi thay đổi API công khai (hàm exported) PHẢI additive/backward-compatible — codebase này có rất nhiều test hiện có gọi trực tiếp các hàm sẽ bị sửa (`ReflectAndExtract`, `NewLearner`, `route()`), xem ghi chú "Không phá test cũ" trong từng task.

---

### Task 1: Routing — parse input dạng "Q: .../A: ..." trước khi match keyword

**Files:**
- Modify: `services/agent-go/internal/orchestrator/orchestrator.go`
- Test: `services/agent-go/internal/orchestrator/orchestrator_test.go`

**Bối cảnh**: `ask_user` render câu hỏi làm rõ; khi user chọn, FE gửi lại dạng `"Q: <câu hỏi JARVIS đặt>\nA: <câu trả lời user>"`. `route()` hiện match keyword trên NGUYÊN câu này — nên keyword như `"tìm hiểu"` trong câu hỏi JARVIS tự đặt ra lại tự làm lạc đề chính JARVIS.

**Step 1: Viết test cho hàm `extractRoutableText` (chưa tồn tại)**

Thêm vào cuối `orchestrator_test.go`:

```go
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
```

**Step 2: Chạy test, xác nhận FAIL (hàm chưa tồn tại)**

Run: `cd services/agent-go && go test ./internal/orchestrator/... -run TestExtractRoutableText -v`
Expected: FAIL với `undefined: extractRoutableText`

**Step 3: Viết implementation**

Thêm vào `orchestrator.go` (sau import, cùng khu vực `asciiWordRe`/`matchTrigger`):

```go
// qaReplyRe khớp input dạng "Q: <câu hỏi>\nA: <câu trả lời>" — format mà FE
// gửi lại khi user trả lời tool ask_user. Bắt buộc bắt đầu bằng "Q:" để
// tránh false positive với câu user gõ tự nhiên có chứa "A: " ở đâu đó.
var qaReplyRe = regexp.MustCompile(`(?s)^Q:\s*.*?\nA:\s*(.*)$`)

// extractRoutableText trả về phần nên dùng để match keyword routing.
//
// Nếu input là reply cho ask_user (dạng "Q: .../A: ..."), CHỈ lấy phần sau
// "A: " — bỏ phần câu hỏi do chính JARVIS đặt ra. Không làm vậy thì keyword
// trong câu hỏi JARVIS tự sinh (vd "tìm hiểu", "giải thích") tự khớp trigger
// của agent khác, khiến JARVIS tự lạc đề khỏi agent đang xử lý hội thoại.
func extractRoutableText(input string) (text string, isReply bool) {
	m := qaReplyRe.FindStringSubmatch(strings.TrimSpace(input))
	if m == nil {
		return input, false
	}
	return strings.TrimSpace(m[1]), true
}
```

**Step 4: Chạy test, xác nhận PASS**

Run: `cd services/agent-go && go test ./internal/orchestrator/... -run TestExtractRoutableText -v`
Expected: PASS (3/3 subtests)

**Step 5: Commit**

```bash
git add services/agent-go/internal/orchestrator/orchestrator.go services/agent-go/internal/orchestrator/orchestrator_test.go
git commit -m "fix(agent-go): tách phần A: khỏi câu hỏi Q: trước khi route keyword"
```

---

### Task 2: Routing — sticky agent theo conversationID

**Files:**
- Modify: `services/agent-go/internal/orchestrator/orchestrator.go`
- Test: `services/agent-go/internal/orchestrator/orchestrator_test.go`

**Không phá test cũ**: `route(input string) *AgentSpec` GIỮ NGUYÊN signature — sticky logic nằm ở `Run()`, gọi `route()` như một bước con. Mọi test cũ gọi `orch.route(tt.input)` trực tiếp (xem `TestOrchestrator_RouteByKeyword`) vẫn chạy nguyên vẹn.

**Step 1: Viết test end-to-end qua `Run()` cho sticky agent**

Thêm vào `orchestrator_test.go` (dùng lại `newFakeEngine` đã có trong file):

```go
// TestOrchestrator_StickyAgentOnReply khoá đúng bug tìm thấy trong log
// production: khi user trả lời câu hỏi ask_user (format "Q:/A:"), câu trả
// lời có thể tình cờ chứa keyword của agent KHÁC (vd "tìm hiểu" khớp agent
// research) — nhưng vì đây là reply cho 1 hội thoại agent "code" đang xử lý,
// PHẢI tiếp tục dùng agent "code", không được nhảy sang "research".
func TestOrchestrator_StickyAgentOnReply(t *testing.T) {
	orch := New()
	orch.Register(&AgentSpec{
		Name:            "code",
		Engine:          newFakeEngine("code", provider.StreamChunk{Kind: provider.ChunkText, Text: "code agent trả lời"}, provider.StreamChunk{Kind: provider.ChunkDone}),
		TriggerKeywords: []string{"code", "repo", "go"},
	})
	orch.Register(&AgentSpec{
		Name:            "research",
		Engine:          newFakeEngine("research", provider.StreamChunk{Kind: provider.ChunkText, Text: "research agent trả lời"}, provider.StreamChunk{Kind: provider.ChunkDone}),
		TriggerKeywords: []string{"tìm hiểu", "research"},
	})

	ctx := context.Background()
	emit := func(agent.Event) {}
	const convID = "conv-sticky-1"

	// Lượt 1: input khớp "code" → agent code xử lý, ghi sticky.
	_, err := orch.Run(ctx, agent.RunInput{ConversationID: convID, UserMessage: "đi sâu vào repo code này", MaxSteps: 2}, emit)
	if err != nil {
		t.Fatalf("Run lượt 1: %v", err)
	}

	// Lượt 2: reply Q:/A: mà phần A: chứa keyword "tìm hiểu" (của research) —
	// PHẢI vẫn ở lại agent "code" nhờ sticky, KHÔNG bị route sang "research".
	var gotAgent string
	emitCapture := func(e agent.Event) {
		if e.Type == "agent" {
			gotAgent = e.Node
		}
	}
	_, err = orch.Run(ctx, agent.RunInput{
		ConversationID: convID,
		UserMessage:    "Q: Bạn muốn tập trung tìm hiểu phần nào?\nA: Core logic",
		MaxSteps:       2,
	}, emitCapture)
	if err != nil {
		t.Fatalf("Run lượt 2: %v", err)
	}
	if gotAgent != "code" {
		t.Errorf("agent lượt 2 = %q, want %q (sticky phải giữ nguyên agent cũ)", gotAgent, "code")
	}
}

// TestOrchestrator_StickyAgentIgnoredWithoutReplyFormat đảm bảo sticky CHỈ áp
// dụng cho reply dạng Q:/A: — câu hỏi mới bình thường (không phải reply) vẫn
// phải route lại theo keyword như cũ, không bị "kẹt" vĩnh viễn ở agent cũ.
func TestOrchestrator_StickyAgentIgnoredWithoutReplyFormat(t *testing.T) {
	orch := New()
	orch.Register(&AgentSpec{
		Name:            "code",
		Engine:          newFakeEngine("code", provider.StreamChunk{Kind: provider.ChunkDone}),
		TriggerKeywords: []string{"code"},
	})
	orch.Register(&AgentSpec{
		Name:            "research",
		Engine:          newFakeEngine("research", provider.StreamChunk{Kind: provider.ChunkDone}),
		TriggerKeywords: []string{"tìm hiểu"},
	})

	ctx := context.Background()
	const convID = "conv-sticky-2"

	_, _ = orch.Run(ctx, agent.RunInput{ConversationID: convID, UserMessage: "sửa code này", MaxSteps: 2}, func(agent.Event) {})

	var gotAgent string
	emitCapture := func(e agent.Event) {
		if e.Type == "agent" {
			gotAgent = e.Node
		}
	}
	// Câu MỚI, không phải format Q:/A: → phải route lại theo keyword, không sticky.
	_, err := orch.Run(ctx, agent.RunInput{ConversationID: convID, UserMessage: "tôi muốn tìm hiểu thêm", MaxSteps: 2}, emitCapture)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if gotAgent != "research" {
		t.Errorf("agent = %q, want %q (câu mới không phải reply, phải route lại)", gotAgent, "research")
	}
}
```

**Step 2: Chạy test, xác nhận FAIL**

Run: `cd services/agent-go && go test ./internal/orchestrator/... -run TestOrchestrator_StickyAgent -v`
Expected: FAIL — agent lượt 2 ở test đầu ra `"research"` thay vì `"code"` (bug thật đang tái hiện).

**Step 3: Viết implementation**

Thêm field vào struct `Orchestrator` (trong `orchestrator.go`, ngay sau field `maxDelegationDepth`):

```go
	stickyAgent map[string]stickyEntry // conversationID → agent đang xử lý hội thoại
	stickyMu    sync.RWMutex
}

// stickyEntry ghi lại agent đang "sở hữu" một hội thoại, để reply cho
// ask_user (format Q:/A:) không bị route lại dựa trên keyword tình cờ xuất
// hiện trong câu trả lời của user.
type stickyEntry struct {
	agentName string
	lastUsed  time.Time
}

// stickyAgentTTL: entry cũ hơn ngưỡng này bị coi như chưa có, route lại từ
// đầu — tránh 1 hội thoại bỏ dở nhiều ngày trước "kẹt cứng" agent cũ mãi mãi.
// Không cần sweep định kỳ: TTL tự áp dụng khi ĐỌC lại entry (xem stickyAgentFor).
const stickyAgentTTL = 24 * time.Hour
```

(Chú ý: cần đóng lại đúng struct `Orchestrator{...}` — chèn 2 field trên vào TRONG dấu `}` đóng struct hiện có, không tạo struct mới.)

Sửa `New()`:

```go
func New() *Orchestrator {
	return &Orchestrator{
		agents:             make(map[string]*AgentSpec),
		maxDelegationDepth: defaultMaxDelegationDepth,
		stickyAgent:        make(map[string]stickyEntry),
	}
}
```

Thêm 2 helper method (đặt gần `route()`):

```go
// stickyAgentFor trả về agent đã ghi cho conversationID này, nếu còn hiệu
// lực (chưa quá TTL và agent đó còn đăng ký trong orchestrator).
func (o *Orchestrator) stickyAgentFor(conversationID string) (*AgentSpec, bool) {
	if conversationID == "" {
		return nil, false
	}
	o.stickyMu.RLock()
	entry, ok := o.stickyAgent[conversationID]
	o.stickyMu.RUnlock()
	if !ok || time.Since(entry.lastUsed) > stickyAgentTTL {
		return nil, false
	}
	spec, exists := o.agents[entry.agentName]
	if !exists {
		return nil, false
	}
	return spec, true
}

// setStickyAgent ghi lại agent vừa xử lý hội thoại này.
func (o *Orchestrator) setStickyAgent(conversationID, agentName string) {
	if conversationID == "" {
		return
	}
	o.stickyMu.Lock()
	o.stickyAgent[conversationID] = stickyEntry{agentName: agentName, lastUsed: time.Now()}
	o.stickyMu.Unlock()
}
```

Sửa `Run()` (thay toàn bộ thân hàm hiện tại):

```go
func (o *Orchestrator) Run(ctx context.Context, in agent.RunInput, emit agent.EmitFunc) (provider.Usage, error) {
	routableText, isReply := extractRoutableText(in.UserMessage)

	var spec *AgentSpec
	// Reply cho ask_user PHẢI tiếp tục agent đang xử lý hội thoại — câu trả
	// lời của user (thường là chọn 1 option) không phải tín hiệu đổi chủ đề,
	// dù tình cờ chứa keyword của agent khác.
	if isReply {
		if sticky, ok := o.stickyAgentFor(in.ConversationID); ok {
			spec = sticky
		}
	}
	if spec == nil {
		spec = o.route(routableText)
	}
	o.setStickyAgent(in.ConversationID, spec.Name)

	slog.Info("orchestrator: routed", "agent", spec.Name, "input_preview", truncate(in.UserMessage, 100))

	emit(agent.Event{
		Type: "agent",
		Node: spec.Name,
	})

	return spec.Engine.Run(ctx, in, emit)
}
```

Thêm import `"sync"` (đã có) và `"time"` vào đầu file nếu chưa có (`time` chưa được import trong `orchestrator.go` — kiểm tra khối `import (...)` ở đầu file và thêm `"time"`).

**Step 4: Chạy test, xác nhận PASS**

Run: `cd services/agent-go && go test ./internal/orchestrator/... -v`
Expected: PASS toàn bộ (bao gồm mọi test cũ — `TestOrchestrator_RouteByKeyword`, `TestOrchestrator_RouteWordBoundary`, ...)

**Step 5: Chạy toàn bộ test suite agent-go để chắc không phá gì khác**

Run: `cd services/agent-go && go build ./... && go test ./...`
Expected: PASS toàn bộ

**Step 6: Commit**

```bash
git add services/agent-go/internal/orchestrator/orchestrator.go services/agent-go/internal/orchestrator/orchestrator_test.go
git commit -m "fix(agent-go): thêm sticky agent theo conversationID, tránh tự lạc đề khi reply ask_user"
```

**⚠️ Checkpoint deploy**: Đây là phần rủi ro cao nhất (theo design doc). Deploy riêng Task 1+2 qua `./deploy/deploy-to-vps.sh`, verify log 1 ngày trước khi làm Task 3.

---

### Task 3: Quota — siết gate `worthLearning()`

**Files:**
- Modify: `services/agent-go/internal/memory/learner_gate.go`
- Test: `services/agent-go/internal/memory/learner_gate_test.go`

**Không phá test cũ**: Test case `"user ngắn nhưng trả lời dài (bài học kỹ thuật) → phải học"` (dòng 60-63 hiện tại, dùng `longAnswer` > 400 rune với user ngắn `"sao chậm?"`) đang PASS nhờ đúng nhánh sắp bị xoá. Sau khi xoá nhánh này, test đó phải đổi kỳ vọng từ `want: true` → `want: false` (đây chính là mục tiêu của fix: KHÔNG học chỉ vì câu trả lời dài).

**Step 1: Sửa test case đã có để phản ánh hành vi MỚI**

Trong `learner_gate_test.go`, sửa case `"user ngắn nhưng trả lời dài..."`:

```go
		{
			name: "user ngắn + trả lời dài nhưng KHÔNG có từ khoá fact → bỏ qua (siết gate)",
			msgs: exchange("sao chậm?", longAnswer),
			want: false, // trước đây true — điều kiện "assistant dài" đã bị bỏ vì gần như luôn đúng, vô hiệu hoá gate
		},
```

Đồng thời thêm case mới xác nhận vẫn học đúng khi có keyword dù trả lời dài:

```go
		{
			name: "user ngắn CÓ từ khoá fact + trả lời dài → vẫn phải học",
			msgs: exchange("tôi thích Go", longAnswer),
			want: true,
		},
```

**Step 2: Chạy test, xác nhận FAIL đúng chỗ**

Run: `cd services/agent-go && go test ./internal/memory/... -run TestWorthLearning -v`
Expected: FAIL ở case `"user ngắn + trả lời dài nhưng KHÔNG có từ khoá fact..."` (code hiện tại vẫn trả `true` vì nhánh cũ chưa xoá)

**Step 3: Sửa implementation**

Trong `learner_gate.go`, hàm `worthLearning`, xoá dòng cuối `return len([]rune(lastAssistant)) > trivialAssistantRunes` và toàn bộ nhánh liên quan:

```go
func worthLearning(messages []provider.Message) bool {
	lastUser, _ := lastByRole(messages)

	if lastUser == "" {
		return false
	}

	lower := strings.ToLower(lastUser)
	for keyword := range keywordToKeys {
		if strings.Contains(lower, keyword) {
			return true
		}
	}

	return len([]rune(lastUser)) > trivialUserRunes
}
```

Xoá hằng số `trivialAssistantRunes` và comment liên quan (dòng khai báo `const (... trivialAssistantRunes = 400)`), cập nhật lại comment phía trên hàm để phản ánh đúng logic mới (bỏ đoạn "Câu trả lời dài (> trivialAssistantRunes) → học").

**Lưu ý**: hàm `lastByRole` vẫn trả về `(user, assistant string)` — giữ nguyên, chỉ không dùng biến `assistant` trong `worthLearning` nữa (dùng `_`). KHÔNG xoá `lastByRole` vì `TestLastByRole_LayTinNhanGanNhat` vẫn test nó trực tiếp.

**Step 4: Chạy test, xác nhận PASS**

Run: `cd services/agent-go && go test ./internal/memory/... -run TestWorthLearning -v`
Expected: PASS toàn bộ subtests

**Step 5: Chạy full test package memory**

Run: `cd services/agent-go && go test ./internal/memory/... -v`
Expected: PASS (kiểm tra `TestLearnFromConversation_KhongGoiProviderKhiTanGau` và `TestLearnFromConversation_VanHocKhiCoFact` vẫn pass vì không phụ thuộc nhánh đã xoá)

**Step 6: Commit**

```bash
git add services/agent-go/internal/memory/learner_gate.go services/agent-go/internal/memory/learner_gate_test.go
git commit -m "fix(agent-go): bỏ điều kiện assistant-dài trong worthLearning — điều kiện gần như luôn đúng nên vô hiệu hoá gate"
```

---

### Task 4: Quota — provider riêng cho reflection (tách khỏi chain Gemini nóng)

**Files:**
- Modify: `services/agent-go/internal/provider/factory/factory.go`
- Modify: `services/agent-go/cmd/server/main.go`
- Test: `services/agent-go/internal/provider/factory/factory_test.go`

**Step 1: Viết test cho `NewReflectionProvider`**

Thêm vào `factory_test.go`:

```go
// NewReflectionProvider phải trả DeepSeek đơn (không bọc chain Gemini) khi có
// DEEPSEEK_API_KEY — reflection là tác vụ nền, không nên cạnh tranh quota
// Gemini với luồng chat chính (bug đã thấy trong log prod: reflection cascade
// qua 6+ biến thể Gemini trước khi rơi xuống DeepSeek).
func TestNewReflectionProvider_PrefersDeepSeek(t *testing.T) {
	cfg := baseCfg()
	cfg.Provider = "auto"
	cfg.GeminiKey = "gk"
	cfg.DeepSeekKey = "dk"
	cfg.AnthropicKey = "ak"

	p, err := NewReflectionProvider(cfg)
	if err != nil {
		t.Fatalf("NewReflectionProvider: %v", err)
	}
	if p.Name() != "deepseek" {
		t.Errorf("Name() = %q, want %q (phải dùng DeepSeek đơn, không chain Gemini)", p.Name(), "deepseek")
	}
}

// Không có DEEPSEEK_API_KEY → fallback về provider chính (factory.New), giữ
// hành vi cũ, không được lỗi hay trả nil.
func TestNewReflectionProvider_FallsBackWithoutDeepSeek(t *testing.T) {
	cfg := baseCfg()
	cfg.Provider = "auto"
	cfg.GeminiKey = "gk"
	cfg.AnthropicKey = "ak"

	p, err := NewReflectionProvider(cfg)
	if err != nil {
		t.Fatalf("NewReflectionProvider: %v", err)
	}
	if p.Name() != "fallback[gemini→anthropic]" {
		t.Errorf("Name() = %q, want fallback[gemini→anthropic] (fallback về provider chính khi thiếu DeepSeek key)", p.Name())
	}
}
```

**Step 2: Chạy test, xác nhận FAIL**

Run: `cd services/agent-go && go test ./internal/provider/factory/... -run TestNewReflectionProvider -v`
Expected: FAIL với `undefined: NewReflectionProvider`

**Step 3: Viết implementation**

Thêm vào `factory.go` (sau hàm `newAuto`):

```go
// NewReflectionProvider tạo provider RIÊNG cho tác vụ reflection nền (trích
// user facts / knowledge items sau mỗi lượt chat — xem internal/memory.Learner).
//
// Vì sao KHÔNG dùng chung provider chính (factory.New): reflection là tác vụ
// phụ trợ, không nên cạnh tranh quota Gemini free-tier với luồng chat chính —
// log production cho thấy reflection cascade qua 6+ biến thể Gemini (429 liên
// tiếp) trước khi rơi xuống DeepSeek, làm chậm và tốn request quota vô ích.
// DeepSeek đơn (rẻ, không rate-limit chặt như Gemini free-tier) là lựa chọn
// hợp lý cho 1 tác vụ trích xuất JSON máy móc, không cần model tốt nhất.
//
// Nếu KHÔNG có DEEPSEEK_API_KEY, fallback về provider chính (factory.New) để
// không phá vỡ khi thiếu key — hành vi giống trước khi có hàm này.
func NewReflectionProvider(cfg config.Config) (provider.Provider, error) {
	if cfg.DeepSeekKey != "" {
		return newDeepSeek(cfg)
	}
	slog.Warn("factory: thiếu DEEPSEEK_API_KEY — reflection dùng chung provider chính, có thể cạnh tranh quota Gemini với chat chính")
	return New(cfg)
}
```

Thêm import `"log/slog"` vào đầu `factory.go` nếu chưa có.

**Step 4: Chạy test, xác nhận PASS**

Run: `cd services/agent-go && go test ./internal/provider/factory/... -v`
Expected: PASS toàn bộ (kể cả test cũ)

**Step 5: Wiring trong `cmd/server/main.go`**

Tìm dòng (khoảng dòng 337, đã xác nhận qua đọc code):

```go
		learner = memory.NewLearner(store, mongoClient, prov, fastModel(cfg), embedder)
```

Đổi thành:

```go
		reflectionProv, err := factory.NewReflectionProvider(cfg)
		if err != nil {
			slog.Warn("main: không tạo được reflection provider riêng, dùng provider chính", "err", err)
			reflectionProv = prov
		}
		learner = memory.NewLearner(store, mongoClient, reflectionProv, fastModel(cfg), embedder)
```

Kiểm tra `factory` package đã được import trong `main.go` (chắc chắn có, vì `factory.New(cfg)` đã dùng ở dòng 45).

**Step 6: Build + test toàn bộ**

Run: `cd services/agent-go && go build ./... && go test ./...`
Expected: PASS

**Step 7: Commit**

```bash
git add services/agent-go/internal/provider/factory/factory.go services/agent-go/internal/provider/factory/factory_test.go services/agent-go/cmd/server/main.go
git commit -m "fix(agent-go): tách provider riêng (DeepSeek) cho reflection, không cạnh tranh quota Gemini với chat chính"
```

---

### Task 5: Quota — batch reflection theo N lượt + co giãn cửa sổ tin nhắn

**Files:**
- Modify: `services/agent-go/internal/memory/learner.go`
- Modify: `services/agent-go/internal/memory/reflection.go`
- Modify: `services/agent-go/internal/config/config.go`
- Modify: `services/agent-go/cmd/server/main.go`
- Test: `services/agent-go/internal/memory/learner_integration_test.go`, `services/agent-go/internal/memory/reflection_test.go`

**Không phá test cũ (QUAN TRỌNG)**:
- `TestLearnFromConversation_KhongGoiProviderKhiTanGau` và `TestLearnFromConversation_VanHocKhiCoFact` (trong `learner_gate_test.go`) gọi `LearnFromConversation` ĐÚNG 1 LẦN và kỳ vọng gọi/không gọi provider NGAY. Nếu batch mặc định N=3 áp dụng vô điều kiện, 2 test này FAIL (vì lượt đầu tiên sẽ bị hoãn).
- Giải quyết: batch turns là **cấu hình theo instance** (default = 1 = không batch, giữ nguyên hành vi cũ), KHÔNG phải hardcode toàn cục. Production set N=3 qua `main.go`; test không set gì → N=1 → hành vi cũ y nguyên.
- `ReflectAndExtract(ctx, p, model, messages)` có **13 call site** hiện có (`reflection_test.go`, `cost_reduction_test.go`, `learner.go`) — KHÔNG đổi signature hàm này. Thêm hàm MỚI `ReflectAndExtractWithWindow(ctx, p, model, messages, windowMessages)`; `ReflectAndExtract` trở thành wrapper mỏng gọi hàm mới với `windowMessages = maxReflectionMessages` (giữ đúng hành vi cũ cho mọi call site hiện có).

**Step 1: Viết test cho cửa sổ co giãn trong `reflection.go`**

Thêm vào `reflection_test.go` (dùng lại `mockP`/helper đã có trong file — đọc file trước khi viết để khớp đúng tên helper hiện có, ví dụ nếu có `newMockProvider` hay tương tự):

```go
// ReflectAndExtractWithWindow phải tôn trọng windowMessages truyền vào, khác
// hằng số cứng maxReflectionMessages — cần khi batch N lượt (N>1) để không
// mất fact của các lượt bị gộp lại (xem Learner.turnCounter).
func TestReflectAndExtractWithWindow_RespectsCustomWindow(t *testing.T) {
	// 6 tin nhắn user/assistant xen kẽ (3 lượt trao đổi) — với window mặc định
	// (4 = 2 lượt) thì lượt ĐẦU TIÊN ("Lượt 1 user"/"Lượt 1 assistant") sẽ bị
	// cắt bỏ; với windowMessages=6 thì phải giữ đủ cả 3 lượt.
	messages := []provider.Message{
		{Role: provider.RoleUser, Content: "Lượt 1 user"},
		{Role: provider.RoleAssistant, Content: "Lượt 1 assistant"},
		{Role: provider.RoleUser, Content: "Lượt 2 user"},
		{Role: provider.RoleAssistant, Content: "Lượt 2 assistant"},
		{Role: provider.RoleUser, Content: "Lượt 3 user"},
		{Role: provider.RoleAssistant, Content: "Lượt 3 assistant"},
	}

	var capturedPrompt string
	mockP := &capturingMockProvider{
		onGenerate: func(req provider.GenerateRequest) {
			capturedPrompt = req.Messages[0].Content
		},
		response: `{"user_facts":[],"knowledge_items":[]}`,
	}

	_, err := ReflectAndExtractWithWindow(context.Background(), mockP, "mock-model", messages, 6)
	if err != nil {
		t.Fatalf("ReflectAndExtractWithWindow: %v", err)
	}
	if !strings.Contains(capturedPrompt, "Lượt 1 user") {
		t.Errorf("prompt thiếu 'Lượt 1 user' — windowMessages=6 phải giữ đủ 3 lượt trao đổi, không cắt về mặc định 4")
	}
}
```

> **Lưu ý cho người thực thi task này**: Đọc `reflection_test.go` TRƯỚC khi viết test trên để dùng đúng kiểu mock provider đã có sẵn trong file (có thể đã có kiểu tương tự `capturingMockProvider` dưới tên khác, hoặc cần thêm field `onGenerate` vào mock hiện có) — không tạo mock trùng lặp nếu file đã có sẵn cơ chế capture request.

**Step 2: Chạy test, xác nhận FAIL**

Run: `cd services/agent-go && go test ./internal/memory/... -run TestReflectAndExtractWithWindow -v`
Expected: FAIL với `undefined: ReflectAndExtractWithWindow`

**Step 3: Refactor `reflection.go`**

Đổi tên logic hiện tại của `ReflectAndExtract` thành `ReflectAndExtractWithWindow` (thêm tham số `windowMessages int`), và biến `ReflectAndExtract` cũ thành wrapper:

```go
// ReflectAndExtract runs a fast LLM pass over conversation messages to extract
// user facts and knowledge items. Dùng cửa sổ mặc định maxReflectionMessages.
// Giữ lại vì có nhiều call site hiện có (test + Learner khi không batch).
func ReflectAndExtract(ctx context.Context, p provider.Provider, model string, messages []provider.Message) (*ReflectionResult, error) {
	return ReflectAndExtractWithWindow(ctx, p, model, messages, maxReflectionMessages)
}

// ReflectAndExtractWithWindow giống ReflectAndExtract nhưng cho phép chỉ định
// số tin nhắn cuối đưa vào prompt (windowMessages) thay vì hằng số cố định
// maxReflectionMessages. Cần khi Learner batch N lượt liền — phải mở cửa sổ
// rộng tương ứng (2*N) để không mất fact của các lượt bị gộp lại, xem
// Learner.turnCounter trong learner.go.
func ReflectAndExtractWithWindow(ctx context.Context, p provider.Provider, model string, messages []provider.Message, windowMessages int) (*ReflectionResult, error) {
	if len(messages) == 0 || p == nil {
		return &ReflectionResult{}, nil
	}
	if windowMessages <= 0 {
		windowMessages = maxReflectionMessages
	}

	var dialogue []provider.Message
	for _, m := range messages {
		if m.Role == provider.RoleUser || m.Role == provider.RoleAssistant {
			dialogue = append(dialogue, m)
		}
	}
	if len(dialogue) > windowMessages {
		dialogue = dialogue[len(dialogue)-windowMessages:]
	}

	// ... (phần còn lại giữ NGUYÊN như ReflectAndExtract cũ: build convText,
	// cắt theo maxReflectionConvRunes, loop reflectOnce với retry — chỉ đổi
	// tên hàm, không đổi logic bên trong)
```

**Bước thực thi thực tế**: copy toàn bộ thân hàm `ReflectAndExtract` hiện tại (dòng 136-201 trong `reflection.go`) sang hàm `ReflectAndExtractWithWindow` mới, đổi 2 chỗ dùng hằng số `maxReflectionMessages` (dòng 150-151 hiện tại) thành tham số `windowMessages`, rồi viết `ReflectAndExtract` mới là 1-liner gọi hàm kia.

**Step 4: Chạy test cửa sổ, xác nhận PASS + chạy lại toàn bộ reflection_test.go**

Run: `cd services/agent-go && go test ./internal/memory/... -run "TestReflectAndExtract" -v`
Expected: PASS toàn bộ — bao gồm mọi test cũ gọi `ReflectAndExtract(...)` 4-arg (không đổi hành vi vì wrapper dùng đúng `maxReflectionMessages` như trước).

**Step 5: Viết test cho batch turn counter trong `Learner`**

Thêm vào `learner_integration_test.go` (đọc file trước để khớp style/helper hiện có — có sẵn `gateSpyProvider`-tương-tự hoặc helper mongo/tenant nào đó):

```go
// TestLearner_BatchTurns_SkipsUntilNthTurn khoá đúng hành vi batch: khi
// SetBatchTurns(3), 2 lượt đầu KHÔNG gọi provider (dù có fact đáng học), chỉ
// lượt thứ 3 mới thực sự chạy reflection.
func TestLearner_BatchTurns_SkipsUntilNthTurn(t *testing.T) {
	spy := &gateSpyProvider{}
	ctx := context.WithValue(context.Background(), middleware.TenantIDKey, "tenant-batch")

	l := NewLearner(NewStore(), nil, spy, "deepseek-v4-flash", nil)
	l.SetBatchTurns(3)

	l.LearnFromConversation(ctx, exchange("tôi tên An", "Chào An!"), "conv-batch")
	l.LearnFromConversation(ctx, exchange("tôi thích Go", "Ghi nhận!"), "conv-batch")
	time.Sleep(150 * time.Millisecond)
	if n := spy.calls.Load(); n != 0 {
		t.Fatalf("provider bị gọi %d lần trước lượt thứ N — batch không hoạt động", n)
	}

	l.LearnFromConversation(ctx, exchange("tôi dùng Postgres", "Ghi nhận!"), "conv-batch")
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if spy.calls.Load() > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Error("lượt thứ N (đủ batch) nhưng provider không được gọi")
}

// Mặc định (không gọi SetBatchTurns) PHẢI giữ hành vi cũ: gọi ngay lượt đầu.
// Đây là test hồi quy quan trọng nhất — bảo vệ 2 test cũ trong
// learner_gate_test.go khỏi bị batch mặc định phá vỡ.
func TestLearner_DefaultBatchTurns_IsOne(t *testing.T) {
	spy := &gateSpyProvider{}
	ctx := context.WithValue(context.Background(), middleware.TenantIDKey, "tenant-default")

	l := NewLearner(NewStore(), nil, spy, "deepseek-v4-flash", nil)
	l.LearnFromConversation(ctx, exchange("tôi tên Bình", "Chào Bình!"), "conv-default")

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if spy.calls.Load() > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Error("batch mặc định phải = 1 (gọi ngay), không được hoãn")
}
```

**Step 6: Chạy test, xác nhận FAIL**

Run: `cd services/agent-go && go test ./internal/memory/... -run TestLearner_ -v`
Expected: FAIL với `undefined: SetBatchTurns` (hoặc tương đương)

**Step 7: Viết implementation trong `learner.go`**

Thêm field + setter vào struct `Learner` và constructor (giữ `NewLearner` signature CŨ nguyên vẹn — không phá call site hiện có):

```go
type Learner struct {
	store       *Store
	mongoClient *mongo.Client
	provider    provider.Provider
	model       string
	embedder    Embedder

	batchTurns int // số lượt gộp lại trước khi thực sự chạy reflection; xem SetBatchTurns
	turnMu     sync.Mutex
	turnCount  map[string]int // conversationID → số lượt đã tích lũy chưa reflect
}

func NewLearner(store *Store, mongoClient *mongo.Client, p provider.Provider, model string, embedder Embedder) *Learner {
	return &Learner{
		store:       store,
		mongoClient: mongoClient,
		provider:    p,
		model:       model,
		embedder:    embedder,
		batchTurns:  1, // mặc định KHÔNG batch — giữ hành vi cũ (chạy ngay mỗi lượt)
		turnCount:   make(map[string]int),
	}
}

// SetBatchTurns đặt số lượt chat gộp lại trước khi thực sự chạy 1 lần
// reflection (giảm số request LLM nền — xem lý do trong design doc
// 2026-08-21-jarvis-reliability-fixes-design.md, mục Quota). n <= 0 → coi
// như 1 (không batch).
func (l *Learner) SetBatchTurns(n int) {
	if n <= 0 {
		n = 1
	}
	l.batchTurns = n
}
```

Thêm import `"sync"` vào đầu `learner.go` nếu chưa có.

Sửa `LearnFromConversation` — chèn logic đếm lượt NGAY SAU gate `worthLearning`, TRƯỚC khi spawn goroutine:

```go
	if !worthLearning(messages) {
		slog.Debug("learner: bỏ qua lượt tán gẫu (không có gì để học)")
		return
	}

	// Batch: gộp N lượt liền trước khi thực sự chạy reflection (giảm số
	// request LLM nền). batchTurns=1 (mặc định) → luôn chạy ngay, giữ đúng
	// hành vi cũ.
	if !l.shouldReflectNow(conversationID) {
		return
	}

	tenantID := middleware.GetTenantID(ctx)
```

Thêm helper method:

```go
// shouldReflectNow tăng bộ đếm lượt của conversationID và trả true khi đã
// đạt batchTurns (rồi reset về 0 cho chu kỳ tiếp theo).
func (l *Learner) shouldReflectNow(conversationID string) bool {
	l.turnMu.Lock()
	defer l.turnMu.Unlock()
	l.turnCount[conversationID]++
	if l.turnCount[conversationID] < l.batchTurns {
		return false
	}
	l.turnCount[conversationID] = 0
	return true
}
```

Sửa lời gọi `ReflectAndExtract` trong goroutine (dòng ~73) thành:

```go
			res, err := ReflectAndExtractWithWindow(bgCtx, l.provider, l.model, msgsCopy, 2*l.batchTurns)
```

**Lưu ý YAGNI (giống `stickyAgent` ở Task 2)**: `turnCount` map không có eviction — chấp nhận được ở quy mô hiện tại, không implement cap/sweep trong đợt này.

**Step 8: Chạy test, xác nhận PASS**

Run: `cd services/agent-go && go test ./internal/memory/... -v`
Expected: PASS toàn bộ, bao gồm 2 test cũ `TestLearnFromConversation_KhongGoiProviderKhiTanGau`/`_VanHocKhiCoFact` (vì `batchTurns` mặc định = 1).

**Step 9: Thêm config + wiring production (N=3 mặc định)**

Trong `internal/config/config.go`, thêm field (gần `DeepSeekProModel`):

```go
	ReflectionBatchTurns int // số lượt chat gộp trước khi chạy reflection. default: 3
```

Trong hàm load config (gần `DeepSeekProModel: envOr(...)`):

```go
		ReflectionBatchTurns: envIntOr("REFLECTION_BATCH_TURNS", 3),
```

Kiểm tra `config.go` đã có helper `envIntOr` (nếu chưa có, thêm hàm nhỏ tương tự `envOr` nhưng parse int qua `strconv.Atoi`, fallback về default khi parse lỗi hoặc rỗng — package đã import `strconv`).

Trong `cmd/server/main.go`, ngay sau dòng tạo `learner` (đã sửa ở Task 4):

```go
		learner = memory.NewLearner(store, mongoClient, reflectionProv, fastModel(cfg), embedder)
		learner.SetBatchTurns(cfg.ReflectionBatchTurns)
```

**Step 10: Build + test toàn bộ + commit**

Run: `cd services/agent-go && go build ./... && go test ./...`
Expected: PASS

```bash
git add services/agent-go/internal/memory/learner.go services/agent-go/internal/memory/reflection.go \
  services/agent-go/internal/memory/learner_integration_test.go services/agent-go/internal/memory/reflection_test.go \
  services/agent-go/internal/config/config.go services/agent-go/cmd/server/main.go
git commit -m "feat(agent-go): batch reflection theo N lượt (default 3) với cửa sổ tin nhắn co giãn"
```

---

### Task 6: Claude 2-key fallback cho chain chat chính

**Files:**
- Modify: `services/agent-go/internal/config/config.go`
- Modify: `services/agent-go/internal/provider/factory/factory.go`
- Test: `services/agent-go/internal/provider/factory/factory_test.go`

**Step 1: Viết test**

Thêm vào `factory_test.go`:

```go
// auto + Claude key thứ 2 → chain thêm 1 tầng Claude, tên phân biệt để dễ
// debug log (anthropic-1/anthropic-2) thay vì trùng tên "anthropic".
func TestNew_AutoWithSecondAnthropicKey(t *testing.T) {
	cfg := baseCfg()
	cfg.Provider = "auto"
	cfg.GeminiKey = "gk"
	cfg.AnthropicKey = "ak1"
	cfg.AnthropicKey2 = "ak2"

	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	want := "fallback[gemini→anthropic-1→anthropic-2]"
	if p.Name() != want {
		t.Errorf("Name() = %q, want %q", p.Name(), want)
	}
}

// Không set AnthropicKey2 → hành vi CŨ y nguyên (backward-compat), tên vẫn
// là "anthropic" (không bị đổi thành "anthropic-1" khi chỉ có 1 key).
func TestNew_AutoWithoutSecondAnthropicKey_Unchanged(t *testing.T) {
	cfg := baseCfg()
	cfg.Provider = "auto"
	cfg.GeminiKey = "gk"
	cfg.AnthropicKey = "ak1"

	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	want := "fallback[gemini→anthropic]"
	if p.Name() != want {
		t.Errorf("Name() = %q, want %q (không set key 2 phải giữ nguyên tên cũ)", p.Name(), want)
	}
}
```

**Step 2: Chạy test, xác nhận FAIL**

Run: `cd services/agent-go && go test ./internal/provider/factory/... -run TestNew_AutoWithSecondAnthropicKey -v`
Expected: FAIL với `cfg.AnthropicKey2 undefined` (field chưa tồn tại trong `config.Config`)

**Step 3: Thêm config**

Trong `internal/config/config.go`, ngay sau field `AnthropicModel` (dòng 27):

```go
	AnthropicKey2        string // Claude key thứ 2, dùng làm fallback tiếp theo nếu key 1 hết quota (optional)
```

Trong hàm load, ngay sau `AnthropicModel: envOr(...)` (dòng 186):

```go
		AnthropicKey2:         os.Getenv("ANTHROPIC_API_KEY_2"),
```

**Step 4: Sửa `factory.go` — thêm Claude key 2 vào chain + wrapper đặt tên**

Thêm type wrapper (đặt gần đầu file, sau import):

```go
// namedOverride bọc 1 provider để đổi Name() hiển thị — dùng khi có NHIỀU
// instance cùng loại provider trong 1 chain (vd 2 Claude key khác nhau) và
// muốn log phân biệt được đang chạy key nào, không đổi gì trong package con
// (anthropic.Client.Name() vẫn trả "anthropic" như cũ).
type namedOverride struct {
	provider.Provider
	name string
}

func (n namedOverride) Name() string { return n.name }
```

Sửa khối `if hasClaude` trong `newAuto()`:

```go
	if hasClaude {
		// Claude key 1 (last resort fallback trước đây).
		c, err := newAnthropic(cfg)
		if err != nil {
			return nil, err
		}
		if cfg.AnthropicKey2 != "" {
			// Có key 2 → đổi tên cả 2 thành anthropic-1/anthropic-2 để log
			// phân biệt được. Không đổi tên khi CHỈ có 1 key (giữ đúng tên
			// "anthropic" cũ, backward-compat với test/log hiện có).
			providers = append(providers, namedOverride{Provider: c, name: "anthropic-1"})

			c2, err := anthropic.New(cfg.AnthropicKey2, cfg.AnthropicModel)
			if err != nil {
				return nil, err
			}
			providers = append(providers, namedOverride{Provider: c2, name: "anthropic-2"})
		} else {
			providers = append(providers, c)
		}
	}
```

Kiểm tra `anthropic` package đã được import trong `factory.go` (đã có, dùng ở `newAnthropic`).

**Step 5: Chạy test, xác nhận PASS**

Run: `cd services/agent-go && go test ./internal/provider/factory/... -v`
Expected: PASS toàn bộ, bao gồm mọi test cũ (`TestNew_AutoChainOrder`, `TestNew_AutoTwoKeys`, ... — không có test nào set `AnthropicKey2` nên đi đúng nhánh `else` giữ tên cũ).

**Step 6: Build + test toàn bộ + commit**

Run: `cd services/agent-go && go build ./... && go test ./...`

```bash
git add services/agent-go/internal/config/config.go services/agent-go/internal/provider/factory/factory.go services/agent-go/internal/provider/factory/factory_test.go
git commit -m "feat(agent-go): thêm Claude API key thứ 2 làm fallback tiếp theo trong chain chat chính"
```

**Cần bạn tự làm sau khi deploy**: set `ANTHROPIC_API_KEY_2` trong `env/.env.production` trên VPS với key Claude thứ 2 — deploy script không tự sinh key này (khác `JWT_SECRET`/`POSTGRES_PASSWORD` ở `env/.env.production.secrets`).

---

### Task 7: Hallucination — thêm rule grounding vào system prompt

**Files:**
- Modify: `services/agent-go/internal/agent/context.go`
- Test: `services/agent-go/internal/agent/context_test.go`

**Step 1: Viết test**

Thêm vào cuối `context_test.go`:

```go
// TestBuildSystemPrompt_GroundingRule khoá rule chống hallucination: model đã
// đọc ĐÚNG tool result (case thật trong log prod: go.mod thật, SHA khớp) vẫn
// bịa ra tech stack khác (Gin/GORM/Viper) vì system prompt trước đây không hề
// yêu cầu bám sát tool output. Xem design doc
// 2026-08-21-jarvis-reliability-fixes-design.md, mục Hallucination.
func TestBuildSystemPrompt_GroundingRule(t *testing.T) {
	prompt := BuildSystemPrompt(nil, nil, "")

	for _, want := range []string{
		"GROUNDING VÀO KẾT QUẢ TOOL",
		"không tìm thấy",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt thiếu %q — rule chống bịa thông tin từ tool result", want)
		}
	}
}
```

**Step 2: Chạy test, xác nhận FAIL**

Run: `cd services/agent-go && go test ./internal/agent/... -run TestBuildSystemPrompt_GroundingRule -v`
Expected: FAIL — prompt chưa chứa "GROUNDING VÀO KẾT QUẢ TOOL"

**Step 3: Thêm rule vào `context.go`**

Trong `BuildSystemPrompt`, chèn bullet mới ngay TRƯỚC dòng:
`b.WriteString("- LÀM RÕ Ý ĐỊNH & LẬP KẾ HOẠCH (BRAINSTORMING / PLANNING / ARCHITECTURE):\n")`

(tức là ngay sau đoạn kết thúc cụm "CHỌN TOOL TRA CỨU LINH HOẠT..." — dòng có nội dung `"...chỉ gọi thêm rag.search khi câu hỏi thực sự nhắc đến ngữ cảnh nội bộ/tài liệu riêng của người dùng.\n")`):

```go
	b.WriteString("- GROUNDING VÀO KẾT QUẢ TOOL (CHỐNG BỊA THÔNG TIN):\n")
	b.WriteString("  + Khi trả lời dựa trên kết quả tool (đọc file, tra cứu code, web.search, rag.search...): BẮT BUỘC chỉ nêu thông tin THẬT SỰ xuất hiện trong tool result đó. TUYỆT ĐỐI KHÔNG tự suy đoán, bổ sung hoặc \"làm đẹp\" thêm chi tiết/thư viện/con số không có trong output — DÙ đó là pattern phổ biến trong kiến thức chung của bạn (vd không tự thêm framework/thư viện vào một dự án chỉ vì đó là combo quen thuộc — phải đúng những gì file cấu hình thật sự chứa).\n")
	b.WriteString("  + Khi tóm tắt/phân tích nội dung file hoặc config lấy từ tool: TRÍCH DẪN đúng đoạn liên quan trong khối mã trước, rồi mới diễn giải dựa trên đúng đoạn đã trích — không diễn giải nội dung chưa có trích dẫn gốc tương ứng.\n")
	b.WriteString("  + Nếu tool result không đủ thông tin để trả lời, PHẢI nói rõ \"không tìm thấy thông tin này trong dữ liệu lấy được\" thay vì đoán.\n")
```

**Step 4: Chạy test, xác nhận PASS**

Run: `cd services/agent-go && go test ./internal/agent/... -v`
Expected: PASS toàn bộ (bao gồm `TestBuildSystemPrompt_NeutralPersona`, `TestBuildSystemPrompt_MentionsRAGList`, ... — rule mới không đụng nội dung các rule cũ)

**Step 5: Build + test toàn bộ + commit**

Run: `cd services/agent-go && go build ./... && go test ./...`

```bash
git add services/agent-go/internal/agent/context.go services/agent-go/internal/agent/context_test.go
git commit -m "fix(agent-go): thêm rule grounding vào system prompt — chống model bịa thông tin ngoài tool result"
```

**Verify tay sau deploy (không thể unit-test hành vi model)**: hỏi lại đúng câu đã gây lỗi gốc ("đi sâu vào services-go của repo" / hỏi tech stack Go), kỳ vọng model trích đúng `go.mod` thật (anthropic-sdk-go, google.golang.org/genai, mongo-driver, sqlite, cron/v3, OpenTelemetry, websocket, gjson/sjson) — KHÔNG bịa Gin/GORM/Viper/Zap.

---

## Sau khi xong tất cả 7 task

1. `cd services/agent-go && go build ./... && go vet ./... && go test ./...` — xanh toàn bộ.
2. Deploy nhóm cuối (Task 6-7) qua `./deploy/deploy-to-vps.sh` từ root repo.
3. Set `ANTHROPIC_API_KEY_2` trong `env/.env.production` trên VPS (nếu chưa set — key thật do user cung cấp).
4. Verify cuối: `ssh hr-vps "docker logs --tail 300 --timestamps jarvis-agent-go"` — không còn `routed agent=research` sai giữa mạch code/general, số dòng `learner: learned...` giảm rõ, không còn cascade 429 qua nhiều biến thể Gemini cho reflection, và test tay câu hỏi gốc không còn bịa tech stack.
