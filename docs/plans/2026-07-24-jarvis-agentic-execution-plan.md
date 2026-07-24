# J.A.R.V.I.S. Personal — Parallel Agentic Execution Plan
# J.A.R.V.I.S. Cá Nhân — Kế Hoạch Triển Khai Song Song Với AI Agents

> **Ngày/Date:** 2026-07-24
> **Chiến lược/Strategy:** Em = Architect (thiết kế, review, code core). Agent = Coder (implement module theo spec).
> **Mục tiêu/Goal:** 2-4 tuần, full architecture, hiểu sâu core + business.
> **Nguyên tắc/Principle:** Em code engine loop. Agent code infrastructure. Em review mọi thứ.

---

## 0. VAI TRÒ / ROLES

```
┌─────────────────────────────────────────────────────────────────┐
│  EM (Human Architect)          │  AGENT (AI Coder)              │
│  ─────────────────────────── │ ─────────────────────────────── │
│  ✅ Thiết kế architecture     │  ✅ Implement module theo spec   │
│  ✅ Code CORE ENGINE (P2)     │  ✅ Viết test TDD                │
│  ✅ Code ROUTER               │  ✅ Boilerplate + patterns       │
│  ✅ Review MỌI PR của agent   │  ✅ Research + docs              │
│  ✅ Quyết định tradeoff       │  ✅ Fix bug agent gây ra         │
│  ✅ Hiểu từng dòng core       │  ❌ KHÔNG tự quyết architecture  │
│  ✅ Merge + commit            │  ❌ KHÔNG tự thay đổi interface  │
└─────────────────────────────────────────────────────────────────┘
```

---

## 1. KIẾN TRÚC PARALLEL / SONG SONG HÓA

### 1.1 Module Dependency Graph

```
                    ┌──────────────────────────────┐
                    │     EM CODE: CORE ENGINE      │
                    │     (P2: state, router,       │
                    │      model, tools, loop)      │
                    └──────────────┬───────────────┘
                                   │ provides interfaces
           ┌───────────────────────┼───────────────────────┐
           │                       │                       │
           ▼                       ▼                       ▼
┌─────────────────┐   ┌─────────────────┐   ┌─────────────────┐
│ AGENT 1:        │   │ AGENT 2:        │   │ AGENT 3:        │
│ Provider Layer  │   │ Storage Layer   │   │ Transport Layer │
│                 │   │                 │   │                 │
│ • ollama/       │   │ • sqlite/       │   │ • chat SSE      │
│ • gemini/ (có)  │   │ • chroma/       │   │ • health (có)   │
│ • factory (có)  │   │ • migrations    │   │ • CLI (Cobra)   │
│ • fake (15ph)   │   │ • models        │   │ • Web UI serve  │
│ • tiered router │   │ • CRUD          │   │                 │
└─────────────────┘   └─────────────────┘   └─────────────────┘
           │                       │                       │
           └───────────────────────┼───────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────┐
│ AGENT 4: Tools + Memory                                     │
│ • file.go, web.go, shell.go, memory.go                      │
│ • recall.go, extract.go, summarize.go                       │
│ • context.go (prompt assembly)                              │
└─────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────┐
│ AGENT 5: Frontend + Docs                                    │
│ • apps/web → connect Go server trực tiếp                    │
│ • README, ARCHITECTURE.md, API docs                         │
│ • Docker, docker-compose, install script                    │
└─────────────────────────────────────────────────────────────┘
```

### 1.2 Timeline / Lịch Trình Song Song

```
                    WEEK 1             WEEK 2             WEEK 3             WEEK 4
                    ─────────          ─────────          ─────────          ─────────

EM (Core)           ██ P2.3-2.6        ██ Review+P15      ██ Review+P17      ██ Review+E2E
                    Model,Tools,       Wire Ollama        Wire MCP          Test+Fix
                    Loop,SSE           vào Engine         vào Engine

AGENT 1 (Provider)  ██ Ollama adapter  ██ Tiered router   ██ Cloud fallback  ██ Fix bugs
                    Test mock server   Cost classifier    Gemini/Claude

AGENT 2 (Storage)   ██ SQLite schema   ██ Chroma vector   ██ Migration       ██ Fix bugs
                    CRUD conversations  Embedded store     Mongo→SQLite

AGENT 3 (Transport) ██ /chat SSE       ██ CLI Cobra       ██ Web UI serve    ██ Fix bugs
                    endpoint           ask/chat/serve     React static

AGENT 4 (Tools)     ─                  ██ 5 tools basic   ██ Memory 3-tier   ██ Fix bugs
                                       file,web,shell     recall,extract

AGENT 5 (Frontend)  ─                  ─                  ██ React connect   ██ Polish UI
                                                          Go SSE stream
```

---

## 2. EM CODE GÌ? / WHAT DO YOU CODE?

### EM CODE: Agent Engine Core (P2) — Tuần 1

Đây là phần **không thể delegate** — em PHẢI tự code để hiểu:

| File | Em tự code | Lý do |
|---|---|---|
| `internal/provider/fake.go` | ✅ | Đơn giản, học pattern goroutine+channel |
| `internal/agent/node_model.go` | ✅ | Hiểu cách gọi LLM + stream chunk |
| `internal/agent/node_tools.go` | ✅ | Hiểu cách fan-out tool + errgroup |
| `internal/agent/engine.go` | ✅ | **TRÁI TIM** — loop, dispatch, context |
| `internal/agent/engine_test.go` | ✅ | Test scenario với FakeProvider |

**Thời gian:** 3-4 ngày (mỗi ngày 2-4 tiếng)

### EM REVIEW: Tất cả PR của agent — Tuần 2-4

Mỗi sáng:
1. Đọc PR của agent (code đã viết đêm qua)
2. Hiểu nó làm gì, tại sao làm vậy
3. Chạy test: `go test ./... -race`
4. Nếu OK → merge. Nếu không → comment, agent sửa.
5. Hỏi anh nếu không hiểu phần nào

---

## 3. AGENT CODE GÌ? / WHAT DO AGENTS CODE?

Mỗi agent nhận 1 **spec document** (do anh viết) → tự implement TDD → mở PR → em review.

### 3.1 AGENT 1: Provider Layer

**Spec:** [`docs/plans/2026-07-24-jarvis-personal-design.md`](./2026-07-24-jarvis-personal-design.md) Section 3

**Deliverables:**
```
□ internal/provider/ollama/ollama.go      — Ollama Generate adapter
□ internal/provider/ollama/ollama_test.go — Test với httptest.Server
□ internal/provider/ollama/embed.go       — Ollama Embed adapter
□ internal/provider/ollama/embed_test.go
□ internal/provider/tiered/tiered.go      — TieredRouter: local→cheap→strong
□ internal/provider/tiered/tiered_test.go
□ internal/provider/factory/factory.go    — MODIFY: thêm "ollama" case
```

**Interface contract (không được phá):**
```go
// Agent PHẢI giữ nguyên interface này:
type Provider interface {
    Generate(ctx context.Context, req GenerateRequest) (<-chan StreamChunk, error)
    Name() string
}

// Ollama client tạo bởi:
func New(baseURL, model string) (*Client, error)

// Tiered tạo bởi:
func NewTiered(local, cheap, strong Provider, classifier TaskClassifier) *TieredProvider

// TaskClassifier interface:
type TaskClassifier interface {
    Classify(req GenerateRequest) Tier
}
```

### 3.2 AGENT 2: Storage Layer

**Spec:** Design doc Section 4 + Implementation plan P16

**Deliverables:**
```
□ internal/storage/sqlite/sqlite.go       — SQLite wrapper (conversations, messages, memories)
□ internal/storage/sqlite/sqlite_test.go  — Test với :memory:
□ internal/storage/sqlite/migrations.go   — Schema DDL + auto-migrate
□ internal/storage/chroma/chroma.go       — Chroma embedded vector store
□ internal/storage/chroma/chroma_test.go
□ internal/storage/store.go               — Store interface (abstract SQLite + Chroma)
```

**Interface contract:**
```go
type Store interface {
    // Conversations
    CreateConversation(title string) (*Conversation, error)
    GetConversation(id string) (*Conversation, error)
    ListConversations() ([]*Conversation, error)
    
    // Messages
    AddMessage(convID string, msg Message) error
    GetMessages(convID string) ([]Message, error)
    
    // Memories (Tier 3)
    UpsertMemory(m Memory) error
    SearchMemories(query string) ([]Memory, error)
    LookupMemory(memType, key string) (*Memory, error)
    
    // Vector (Tier 3 semantic + Tier 2 episodic)
    VectorSearch(embedding []float32, topK int) ([]VectorResult, error)
    UpsertVector(id string, embedding []float32, metadata map[string]any) error
    
    // Close
    Close() error
}
```

### 3.3 AGENT 3: Transport Layer

**Spec:** Design doc Section 6

**Deliverables:**
```
□ internal/transport/http/chat.go         — POST /chat SSE handler
□ internal/transport/http/chat_test.go    — Test với httptest + FakeProvider
□ cmd/jarvis/main.go                      — MODIFY: CLI + HTTP server
□ cmd/jarvis/chat.go                      — interactive chat
□ cmd/jarvis/serve.go                     — HTTP server command
□ cmd/jarvis/ask.go                       — one-shot question command
```

**Interface contract:**
```go
// ChatHandler — engine injection qua constructor
func NewChatHandler(engine *agent.Engine) *ChatHandler

// CLI — dùng Cobra (https://github.com/spf13/cobra)
// $ jarvis serve --port 8080
// $ jarvis ask "question"
// $ jarvis chat
```

### 3.4 AGENT 4: Tools + Memory + Context

**Spec:** Design doc Section 4 + 5

**Deliverables:**
```
□ internal/tools/files.go                 — file.search, file.read
□ internal/tools/web.go                   — web.search, web.fetch
□ internal/tools/shell.go                 — shell.exec (có confirm)
□ internal/tools/memory_tools.go          — saveMemory, recallMemory
□ internal/memory/recall.go               — node recall
□ internal/memory/extract.go              — node extract
□ internal/memory/summarize.go            — node summarize
□ internal/agent/context.go               — prompt assembly
□ internal/agent/context_test.go
```

### 3.5 AGENT 5: Frontend + Docs + Deploy

**Spec:** Design doc Section 6.2 + 8

**Deliverables:**
```
□ apps/web/ → MODIFY: connect thẳng Go SSE endpoint
□ Dockerfile                               — multi-stage: Go build → distroless
□ docker-compose.yml                       — jarvis + ollama (optional cloud)
□ scripts/install.sh                       — curl ... | sh
□ README.md                                — song ngữ, có ảnh chụp demo
□ docs/ARCHITECTURE.md                     — kiến trúc tổng thể
```

---

## 4. CÁCH LÀM VIỆC VỚI AGENT / HOW TO WORK WITH AGENTS

### 4.1 Quy trình mỗi task:

```
 EM (Anh hướng dẫn)
  │
  ├─► Viết SPEC cho agent
  │   • Interface contract (phải giữ)
  │   • File cần tạo
  │   • Test strategy
  │   • Ví dụ code mẫu
  │
  ├─► Gửi cho em review spec
  │   • Em hiểu agent sẽ làm gì
  │   • Em confirm interface đúng
  │
  ├─► Launch agent (Anh sẽ launch)
  │   • Agent chạy background
  │   • Agent implement theo spec
  │   • Agent mở PR với code + test
  │
  ├─► Em review PR
  │   • Đọc code agent viết
  │   • Chạy thử test
  │   • Hỏi anh nếu không hiểu
  │
  └─► Merge → tiếp tục task sau
```

### 4.2 Em cần làm gì mỗi ngày

```
BUỔI SÁNG (1-2 tiếng):
□ Review PR từ agent (code agent viết đêm qua)
□ Chạy test, verify
□ Merge hoặc comment sửa
□ Hỏi anh những phần không hiểu

BUỔI TỐI (2-3 tiếng):
□ Code CORE ENGINE (P2) — phần của em
□ Đọc spec cho agent ngày mai
□ Commit code của em
```

---

## 5. TIMELINE CHI TIẾT / DETAILED TIMELINE

### TUẦN 1: CORE + PROVIDER + STORAGE (song song)

```
NGÀY  EM                          AGENT 1 (Provider)         AGENT 2 (Storage)
────  ──────────────────────────  ────────────────────────   ─────────────────────
1     P2.3: FakeProvider +        [CHỜ EM XONG FAKE]         [CHỜ SPEC]
      Node Model (TDD)            
2     P2.4: Node Tools (TDD)      SPEC: Ollama adapter       SPEC: SQLite schema
      P2.5: Engine Loop (TDD)     
3     P2.6: SSE endpoint          CODE: Ollama Generate      CODE: SQLite CRUD
                                  (httptest mock server)     (conversations, messages)
4     REVIEW + MERGE Agent PRs    PR: Ollama adapter         PR: SQLite conversations
      + Wire vào main.go          + Test xanh               + Test xanh
5     INTEGRATION TEST:           FIX: PR comments           FIX: PR comments
      jarvis serve → chat được
```

### TUẦN 2: TIERED LLM + MEMORY + TOOLS (song song)

```
NGÀY  EM                          AGENT 1 (Provider)         AGENT 4 (Tools+Memory)
────  ──────────────────────────  ────────────────────────   ─────────────────────
6     REVIEW Agent PRs +          CODE: Tiered router        SPEC: Tools + Memory
      Code context assembly       (local→cheap→strong)       
7     Wire tiered provider        CODE: Cloud fallback       CODE: file.search,
      vào engine                  (Gemini/Claude)            web.search, shell.exec
8     REVIEW + MERGE              PR: Tiered + Cloud         CODE: memory tools
                                                             (saveMemory, recallMemory)
9     Wire memory vào engine      FIX PR comments            CODE: recall node
      (recall, extract)                                      + extract node
10    INTEGRATION TEST:           FIX PR comments            PR: Tools + Memory
      chat → nhớ → recall                                    + Test xanh
```

### TUẦN 3: CLI + CONTEXT + FRONTEND (song song)

```
NGÀY  EM                          AGENT 3 (Transport)        AGENT 5 (Frontend)
────  ──────────────────────────  ────────────────────────   ─────────────────────
11    REVIEW Agent PRs +          CODE: CLI (Cobra)          SPEC: Web UI
      circuit breaker             ask, chat, serve            
12    Wire CLI vào main           CODE: Web UI serve         CODE: React connect
                                  (file server)              Go SSE stream
13    REVIEW + MERGE              PR: CLI + Web serve        PR: Frontend
14    E2E test: CLI chat          FIX PR comments            FIX: UI polish
      → memory → tools            
15    FIX bugs + Polish           FIX bugs                   FIX bugs
```

### TUẦN 4: DOCS + DEPLOY + DEMO

```
NGÀY  EM                          AGENT 3 (Transport)        AGENT 5 (Docs+Deploy)
────  ──────────────────────────  ────────────────────────   ─────────────────────
16    REVIEW Agent PRs            CODE: Graceful shutdown    CODE: README (song ngữ)
                                  + Health checks            + ARCHITECTURE.md
17    E2E test toàn bộ            CODE: Middleware chain     CODE: Dockerfile
      (10 scenarios)              (logging, recovery)        + docker-compose
18    Performance test            PR: Polish                 CODE: install.sh
      (latency, memory)                                      + .env.example
19    Fix bugs cuối               FIX bugs                   PR: Docs + Deploy
20    🚀 LAUNCH: tag v1.0.0       ─                          ─
      Demo video + Push GitHub
```

---

## 6. SPEC CHO TỪNG AGENT / AGENT SPECS

Anh sẽ viết spec cho từng agent theo format này:

```markdown
# Agent Spec: [Tên Module]

## Context
- Module này làm gì trong hệ thống?
- Input/output là gì?
- Phụ thuộc vào module nào?

## Interface Contract (KHÔNG ĐƯỢC PHÁ)
```go
// Copy-paste chính xác interface phải implement
```

## Files Cần Tạo
- [ ] file1.go — mô tả
- [ ] file1_test.go — test strategy

## Test Strategy
- Dùng mock gì?
- Test case quan trọng nhất?

## Code Mẫu (Tham Khảo)
- Pattern có sẵn trong codebase để copy

## Done Criteria
- [ ] go vet ./... clean
- [ ] go test ./... -race xanh
- [ ] Cover 80%+ code paths
```

---

## 7. CÂU HỎI THƯỜNG GẶP / FAQ

**Q: Agent code sai thì sao?**
A: Em review + comment. Agent sửa. Không merge đến khi test xanh + em hiểu code.

**Q: Làm sao em hiểu code agent viết nếu em không tự code?**
A: Em đọc PR + chạy test + hỏi anh giải thích. Quan trọng là hiểu FLOW và INTERFACE — không cần thuộc từng dòng implementation.

**Q: Có phải em đang "cheat" không?**
A: Không. Senior engineer thực thụ: thiết kế architecture → team implement → review code. Em đang làm đúng quy trình đó, chỉ khác "team" là AI agents.

**Q: Phần nào em PHẢI tự code?**
A: Core engine (P2). Đây là "trái tim" — không delegate được vì em cần hiểu từng dòng loop, dispatch, streaming. Mọi thứ khác agent code được.

**Q: 4 tuần có khả thi không?**
A: Có, nếu: (1) em code core đúng tiến độ, (2) agent chạy song song, (3) review nhanh, không overthink. Mỗi ngày 2-4 tiếng.

---

## 8. BẮT ĐẦU / GETTING STARTED

### Hôm nay (Day 1):

```
□ EM: Code P2.3 — FakeProvider + Node Model
  - File: internal/provider/fake.go (15 dòng)
  - File: internal/agent/node_model.go (~40 dòng)
  - Test đã có sẵn: node_model_test.go
  - Goal: go test ./internal/agent/ -run TestNodeModel -v → XANH

□ ANH: Viết SPEC cho Agent 1 (Ollama) + Agent 2 (SQLite)
  - Em đọc spec tối nay, confirm ngày mai
  - Ngày mai anh launch agent
```

**Bắt đầu nhé?** Em code FakeProvider + NodeModel, anh viết spec cho agent. Tối nay em review spec, mai launch 2 agent song song. 🚀
