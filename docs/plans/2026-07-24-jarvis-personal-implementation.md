# J.A.R.V.I.S. Personal — Implementation Plan
# J.A.R.V.I.S. Cá Nhân — Kế Hoạch Triển Khai

> **Ngày/Date:** 2026-07-24
> **Design doc:** [`2026-07-24-jarvis-personal-design.md`](./2026-07-24-jarvis-personal-design.md)
> **Base branch:** `feat/go-agent` (P0-P2 đang triển khai)
> **Strategy:** Build trên nền tảng hiện có, thay đổi infrastructure layer (MongoDB→SQLite, Voyage→Ollama), thêm personal features.

---

## 0. Strategy / Chiến Lược

### 0.1 Nguyên Tắc Triển Khai / Implementation Principles

| # | Nguyên tắc / Principle | Mô tả / Description |
|---|---|---|
| I1 | **Build on existing** | Giữ toàn bộ engine (P0-P14), chỉ đổi infrastructure + thêm features |
| I2 | **TDD every task** | Viết test đỏ → code → xanh → commit. Không ngoại lệ. |
| I3 | **Vertical slices** | Mỗi phase cho ra 1 thứ DÙNG ĐƯỢC (không phải "nền móng xong hết rồi mới dùng") |
| I4 | **Local-first** | Mọi thứ chạy local trước, cloud là optional add-on |
| I5 | **Single binary** | `go build` → 1 file `jarvis` là chạy được |
| I6 | **Go only** (no TS gateway) | Xóa `apps/api`, Go serve HTTP/SSE trực tiếp |
| I7 | **Commit often** | Mỗi task = 1 commit, message tiếng Anh chuẩn conventional commits |

### 0.2 Cách Chạy / How To Run

```bash
# Mọi lệnh chạy từ thư mục services/jarvis (đổi tên từ services/agent-go)
cd services/jarvis

# Build
go build -o bin/jarvis ./cmd/jarvis

# Test
go test ./... -race

# Run
./bin/jarvis serve

# Dev (hot reload)
air
```

---

## PHASE MAP: TỪ HIỆN TẠI ĐẾN JARVIS V1

```
Current: P0-P2 (feat/go-agent) → JARVIS v1

P0-P2   ████████░░░░░░░░░░  Agent Engine (đang làm)
P15     ░░░░░░░░░░░░░░░░░░  Ollama Integration
P16     ░░░░░░░░░░░░░░░░░░  SQLite Migration
P3-P9   ░░░░░░░░░░░░░░░░░░  Tools + Memory + Context (tái sử dụng, đổi backend)
P17     ░░░░░░░░░░░░░░░░░░  MCP Device Layer
P10-P13 ░░░░░░░░░░░░░░░░░░  Guardrails + Obs + Eval
P18     ░░░░░░░░░░░░░░░░░░  CLI Interface
P19     ░░░░░░░░░░░░░░░░░░  Proactive Engine
P20     ░░░░░░░░░░░░░░░░░░  Personality Engine
P14     ░░░░░░░░░░░░░░░░░░  Polish + Docs
P21-P22 ░░░░░░░░░░░░░░░░░░  Desktop + Mobile (optional)
```

---

## PHASE 0-2: CORE ENGINE (ĐANG LÀM — HOÀN THIỆN TRƯỚC)

### P0 — Scaffold + CI ✅ DONE

Giữ nguyên từ `feat/go-agent`. Chỉ đổi tên module: `agent-go` → `jarvis`.

### P1 — Provider Layer ✅ DONE (+ Thêm Ollama ở P15)

Giữ nguyên. Thêm `internal/provider/ollama/` ở P15.

### P2 — Agent Engine 🔄 IN PROGRESS

**Mục tiêu / Goal:** Hoàn thiện engine lõi với ReAct loop.

| Task | Status | Ghi chú |
|---|---|---|
| P2.1 State + Event types | ✅ | Thêm `UserContext` field |
| P2.2 Router (pure function) | ✅ | |
| P2.3 Node model (LLM call) | 🔜 | Em đang làm |
| P2.4 Node tools (fan-out) | 📋 | |
| P2.5 Engine Run loop | 📋 | |
| P2.6 SSE /chat endpoint | 📋 | |

---

## PHASE 15: OLLAMA INTEGRATION (MỚI — SAU P2)

### P15.1 — Ollama Provider Adapter (TDD)

**Mục tiêu / Goal:** Ollama thành provider thứ 3 (cạnh Gemini, Claude). Gọi local LLM qua REST API.

**Files:**
- Create: `internal/provider/ollama/ollama.go` — adapter chính
- Create: `internal/provider/ollama/ollama_test.go` — test với `httptest.Server`
- Create: `internal/provider/ollama/embed.go` — embedding adapter
- Modify: `internal/provider/factory/factory.go` — thêm `"ollama"` case

**Ollama API cần gọi / API endpoints to call:**

```go
// Chat (streaming)
POST http://localhost:11434/api/chat
{
    "model": "llama3.1:8b",
    "messages": [
        {"role": "user", "content": "Hello"}
    ],
    "tools": [...],      // Ollama hỗ trợ tool calling từ 0.3+
    "stream": true,
    "options": {
        "temperature": 0.7,
        "num_predict": 4096
    }
}
// Response: từng dòng JSON {"message": {"role": "assistant", "content": "Hi"}, "done": false}

// Embedding
POST http://localhost:11434/api/embed
{
    "model": "nomic-embed-text",
    "input": ["text to embed"]
}
// Response: {"embeddings": [[0.1, 0.2, ...]]}
```

**Test strategy / Chiến lược test:**

```go
// Dùng httptest.Server giả lập Ollama API → không cần Ollama thật để test
func TestOllamaGenerate(t *testing.T) {
    // 1. Tạo fake Ollama server (httptest.Server)
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 2. Verify request format
        // 3. Trả về SSE chunks giả
        fmt.Fprintf(w, `{"message":{"role":"assistant","content":"Hello"},"done":false}`)
        fmt.Fprintf(w, `{"message":{"role":"assistant","content":" World"},"done":false}`)
        fmt.Fprintf(w, `{"message":{"role":"assistant","content":""},"done":true}`)
    }))
    defer server.Close()

    // 4. Tạo Ollama client trỏ đến fake server
    client := NewWithBaseURL(server.URL, "llama3.1:8b")
    
    // 5. Gọi Generate → assert stream chunks
    req := provider.GenerateRequest{...}
    stream, err := client.Generate(ctx, req)
    // Assert chunks: 2 text chunks + done
}
```

**Hàm dịch thuần (testable without network):**

```go
// toOllamaMessages: provider.Message → Ollama API format
func toOllamaMessages(msgs []provider.Message) []ollamaMessage { ... }

// fromOllamaChunk: Ollama SSE line → provider.StreamChunk
func fromOllamaChunk(line []byte) (provider.StreamChunk, bool) { ... }

// toOllamaTools: provider.ToolDef → Ollama tool format
func toOllamaTools(tools []provider.ToolDef) []ollamaTool { ... }
```

**Done criteria / Tiêu chí hoàn thành:**
- [ ] `ollama.New()` tạo client, ping `/api/tags` verify Ollama đang chạy
- [ ] `Generate()` trả về `<-chan StreamChunk` giống Gemini/Claude adapter
- [ ] `Embed()` trả về `[][]float32` cho RAG
- [ ] Test: mock server + translation functions TDD
- [ ] Factory: `provider=factory.New(cfg)` chọn được `ollama`
- [ ] Integration test manual: `go test -tags=integration -run=TestOllamaReal` (nếu có GPU)

### P15.2 — Tiered Provider Router

**Mục tiêu / Goal:** Tự động chọn provider dựa trên độ phức tạp của task.

**Files:**
- Create: `internal/provider/tiered/tiered.go`
- Create: `internal/provider/tiered/classifier.go`

**Logic / Logic:**

```go
type Tier int
const (
    TierLocal  Tier = iota  // Ollama — rẻ, private
    TierCheap               // Gemini Flash Lite — nhanh, rẻ
    TierStrong              // Claude Opus — mạnh, đắt
)

func (t *Tiered) Classify(req provider.GenerateRequest) Tier {
    // Heuristic đơn giản (P15) → classifier model (P20):
    // - Tin nhắn ngắn, không tool → Local
    // - Có tool calls, context dài → Cheap
    // - Có instruction phức tạp, code gen → Strong
    //
    // Default: Local → fallback Cheap → fallback Strong
}
```

**Done criteria / Tiêu chí hoàn thành:**
- [ ] Classify dựa trên heuristic (số message, có tool không, độ dài)
- [ ] Fallback chain: local → cheap → strong
- [ ] Test: mock providers, assert chọn đúng tier

---

## PHASE 16: SQLITE MIGRATION (THAY MONGODB)

### P16.1 — SQLite Store Interface

**Mục tiêu / Goal:** Thay toàn bộ `internal/mongo/` bằng SQLite + Chroma.

**Files:**
- Create: `internal/storage/sqlite/sqlite.go` — SQLite wrapper
- Create: `internal/storage/sqlite/migrations.go` — schema migrations
- Create: `internal/storage/sqlite/sqlite_test.go`
- Create: `internal/storage/chroma/chroma.go` — Chroma embedded
- Create: `internal/storage/chroma/chroma_test.go`
- Remove: `internal/mongo/` (giữ lại branch cũ để reference)

**Schema / Lược đồ CSDL:**

```sql
-- conversations: lưu chat history
CREATE TABLE conversations (
    id TEXT PRIMARY KEY,
    title TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    message_count INTEGER DEFAULT 0,
    summary TEXT  -- auto-generated summary (Tier 2 memory)
);

-- messages: từng tin nhắn trong conversation
CREATE TABLE messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id TEXT NOT NULL REFERENCES conversations(id),
    role TEXT NOT NULL CHECK (role IN ('user', 'assistant', 'system', 'tool')),
    content TEXT,
    tool_calls JSON,       -- JSON array of {id, name, args}
    tool_call_id TEXT,      -- for role='tool'
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_messages_conv ON messages(conversation_id, created_at);

-- memories: Tier 3 semantic memory
CREATE TABLE memories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL CHECK (type IN ('preference', 'fact', 'entity', 'relationship')),
    key TEXT NOT NULL,
    value TEXT NOT NULL,
    confidence REAL DEFAULT 1.0 CHECK (confidence >= 0.0 AND confidence <= 1.0),
    source TEXT DEFAULT 'ai_extracted' CHECK (source IN ('ai_extracted', 'manual', 'observed')),
    conversation_id TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    access_count INTEGER DEFAULT 0,
    UNIQUE(type, key)  -- dedup: keep highest confidence
);
CREATE INDEX idx_memories_type ON memories(type);

-- procedures: Tier 4 procedural memory
CREATE TABLE procedures (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    trigger_keywords JSON,     -- ["standup", "daily update"]
    steps JSON,                -- [{"order": 1, "action": "...", "tool": "..."}]
    success_rate REAL DEFAULT 0.0,
    use_count INTEGER DEFAULT 0,
    last_used DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- episodes: Tier 2 episodic memory (compressed conversations)
CREATE TABLE episodes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id TEXT NOT NULL,
    summary TEXT NOT NULL,
    key_moments JSON,          -- [{"time": "...", "event": "..."}]
    tags JSON,                 -- ["coding", "debug", "meeting"]
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

**Test strategy / Chiến lược test:**

```go
func TestSQLiteConversations(t *testing.T) {
    // Dùng :memory: database → không cần file thật
    store, err := sqlite.Open(":memory:")
    require.NoError(t, err)
    defer store.Close()

    // Test CRUD
    conv, err := store.CreateConversation("Test Chat")
    msgs, err := store.GetMessages(conv.ID)
    // ... assertions
}
```

**Done criteria / Tiêu chí hoàn thành:**
- [ ] SQLite schema được tạo tự động (auto-migrate on startup)
- [ ] CRUD conversations + messages (thay thế `apps/api` TS code)
- [ ] CRUD memories với dedup (type+key unique, keep highest confidence)
- [ ] CRUD procedures
- [ ] Chroma embedded chạy (không cần external process)
- [ ] Full-text search trên messages (SQLite FTS5)
- [ ] Test: `:memory:` database, không cần file thật

### P16.2 — Data Migration (Export Từ MongoDB)

```bash
# Script 1 lần: export MongoDB → JSON → import SQLite
$ jarvis migrate --from-mongo "mongodb+srv://..." --to-sqlite "~/.jarvis/data/jarvis.db"
```

---

## PHASE 3-9: TOOLS + MEMORY + CONTEXT (TÁI SỬ DỤNG, ĐỔI BACKEND)

Các phase này giữ logic ENGINE, đổi backend data store:

| Original Phase | What Changes | What Stays |
|---|---|---|
| P3 Tool System | Tools đọc/ghi SQLite thay vì MongoDB | `Tool` interface, `Registry`, `RunParallel` |
| P4 Mongo | → **P16 SQLite** (đã thay) | |
| P5 RAG | Voyage → **Ollama embedding** | RAG tools (`ragSearch`, `readDocument`) |
| P6 Context Engineering | Thêm personal context (UserContext) | Prompt assembly order |
| P7 Memory 3-tier → **4-tier** | SQLite memories + Chroma vectors + procedures table | `recall`/`summarize`/`extract` nodes |
| P8 Planner + Reflection | Giữ nguyên | Node logic |
| P9 Skills | Load từ `~/.jarvis/skills/` | Progressive disclosure pattern |

---

## PHASE 17: MCP DEVICE LAYER (MỚI)

### P17.1 — MCP Client + Auto-Discovery

**Mục tiêu / Goal:** JARVIS tự động tìm và kết nối đến MCP servers trên máy.

**Files:**
- Create: `internal/tools/mcp/client.go` — MCP stdio/HTTP client
- Create: `internal/tools/mcp/discovery.go` — auto-discover từ config
- Create: `internal/tools/mcp/registry.go` — MCP tool → Tool interface adapter

**Config format / Định dạng config:**

```yaml
# ~/.jarvis/mcp-servers/email.yaml
name: email
command: node
args: ["~/jarvis/mcp-servers/email-server/dist/index.js"]
env:
  IMAP_HOST: imap.gmail.com
  IMAP_USER: tony@starkindustries.com
  # Password từ keychain, không lưu plaintext

---
# ~/.jarvis/mcp-servers/github.yaml
name: github
command: npx
args: ["-y", "@modelcontextprotocol/server-github"]
env:
  GITHUB_PERSONAL_ACCESS_TOKEN: ${GITHUB_TOKEN}  # từ env var
```

**Done criteria / Tiêu chí hoàn thành:**
- [ ] Đọc tất cả YAML config từ `~/.jarvis/mcp-servers/`
- [ ] Kết nối MCP server qua stdio (subprocess) hoặc HTTP (localhost)
- [ ] Gọi `tools/list` → đăng ký vào Tool Registry
- [ ] Gọi `tools/call` → chạy tool
- [ ] Test: mock MCP server process

### P17.2 — Personal Tools Implementation

**Mục tiêu / Goal:** Implement các tool personal: email, calendar, contacts, notes, browser, health.

**Files (mỗi tool 1 file + test):**
- Create: `internal/tools/personal/email.go` + test
- Create: `internal/tools/personal/calendar.go` + test
- Create: `internal/tools/personal/contacts.go` + test
- Create: `internal/tools/personal/notes.go` + test
- Create: `internal/tools/personal/browser.go` + test
- Create: `internal/tools/personal/health.go` + test

**Tool design pattern (mỗi tool giống nhau):**

```go
// EmailTool — tìm và đọc email local
type EmailTool struct {
    index *EmailIndex  // local IMAP index (SQLite FTS5)
    smtp  *SMTPClient  // for sending
}

func (t *EmailTool) Name() string { return "email.search" }
func (t *EmailTool) Description() string { return "Tìm email trong hộp thư (Search emails in your inbox)" }
func (t *EmailTool) Schema() json.RawMessage {
    return json.RawMessage(`{
        "type": "object",
        "properties": {
            "query": {"type": "string", "description": "Search query"},
            "from": {"type": "string", "description": "Filter by sender"},
            "limit": {"type": "integer", "default": 10}
        },
        "required": ["query"]
    }`)
}
func (t *EmailTool) Kind() tools.Kind { return tools.KindRead }  // search = safe
func (t *EmailTool) Execute(ctx context.Context, args json.RawMessage) (tools.Result, error) {
    // Parse args → search local IMAP index → return results
}
```

**Done criteria / Tiêu chí hoàn thành:**
- [ ] Mỗi tool có test với mock data
- [ ] Email: search + send (có guardrail confirm trước khi gửi)
- [ ] Calendar: list + create (dùng iCal format)
- [ ] Notes: search + create (tương thích Obsidian/Markdown)
- [ ] Browser: search history (đọc SQLite của Chrome/Firefox/Safari)

---

## PHASE 10-13: GUARDRAILS + OBS + EVAL

Giữ nguyên thiết kế, đơn giản hóa cho single-user:

| Phase | What Changes for Personal |
|---|---|
| P10 Guardrails | Bỏ HITL (single user, bạn toàn quyền). Giữ circuit breaker + input validation. |
| P11 Observability | Local OTel (không cần cloud backend). Slog → file. |
| P12 Gateway | **BỎ** — không cần `apps/api`. Go serve HTTP/SSE trực tiếp. |
| P13 Eval | Giữ LLM-as-judge. Thêm personal eval set của bạn. |

---

## PHASE 18: CLI INTERFACE (MỚI)

### P18.1 — CLI với Cobra

**Mục tiêu / Goal:** `jarvis` command với subcommands: `chat`, `serve`, `memory`, `config`.

**Files:**
- Create: `cmd/jarvis/main.go` — entry point
- Create: `cmd/jarvis/chat.go` — interactive chat
- Create: `cmd/jarvis/serve.go` — HTTP server
- Create: `cmd/jarvis/memory.go` — memory management
- Create: `cmd/jarvis/config.go` — config management

**CLI examples / Ví dụ CLI:**

```bash
# Quick question (single-turn)
$ jarvis ask "What's on my calendar today?"

# Interactive chat (multi-turn)
$ jarvis chat

# Start server (background)
$ jarvis serve --port 8080

# Memory management
$ jarvis memory list                    # list all memories
$ jarvis memory add --type preference --key coffee --value "black, no sugar"
$ jarvis memory search "coffee"        # search memories

# Config
$ jarvis config show                    # show current config
$ jarvis config set llm.local.model "llama3.1:70b"

# Device management
$ jarvis devices list                   # list connected MCP devices
$ jarvis devices add ./my-tool.yaml     # add new MCP server

# Backup/Restore
$ jarvis backup                         # backup to ~/.jarvis/backups/
$ jarvis restore ~/.jarvis/backups/jarvis-2026-07-24.db.gz
```

**Done criteria / Tiêu chí hoàn thành:**
- [ ] `jarvis ask "question"` — gửi 1 câu, nhận 1 câu trả lời
- [ ] `jarvis chat` — interactive chat với streaming
- [ ] `jarvis serve` — start HTTP server + WebSocket
- [ ] `jarvis memory *` — CRUD memories từ CLI
- [ ] `jarvis config *` — đọc/ghi config YAML
- [ ] Shell completion: `jarvis completion zsh`

---

## PHASE 19: PROACTIVE ENGINE (MỚI)

### P19.1 — Scheduled + Event-Driven Watchers

**Mục tiêu / Goal:** JARVIS không đợi bạn hỏi — nó chủ động thông báo.

**Files:**
- Create: `internal/proactive/scheduler.go` — cron-based scheduled tasks
- Create: `internal/proactive/watchers.go` — event-driven triggers
- Create: `internal/proactive/notifications.go` — desktop notification

**Ví dụ / Examples:**

```go
// Morning briefing — 7:00 AM mỗi ngày
scheduler.Add(ScheduledTask{
    Name:     "morning_briefing",
    Cron:     "0 7 * * *",
    Action: func(ctx context.Context) {
        summary := jarvis.generateBriefing(ctx)
        notify.Send("Good morning!", summary)
    },
})

// Email from VIP — instant notification
watcher.Watch(FileChangeTrigger{
    Path:    "~/.jarvis/data/email-index.db",
    Pattern: "INSERT WHERE sender IN (SELECT email FROM contacts WHERE vip = true)",
    Action: func(ctx context.Context, event Event) {
        notify.Send("📧 Email from Pepper", event.Summary)
    },
})
```

**Done criteria / Tiêu chí hoàn thành:**
- [ ] Scheduled tasks chạy đúng giờ (dùng `robfig/cron`)
- [ ] Event watchers: file change, new email, calendar reminder
- [ ] Desktop notification (dùng `beeep` hoặc osascript)

---

## PHASE 20: PERSONALITY ENGINE (MỚI)

### P20.1 — Learn & Adapt

**Mục tiêu / Goal:** JARVIS học giọng văn, thói quen, sở thích của bạn.

**Files:**
- Create: `internal/personality/profile.go` — user profile
- Create: `internal/personality/tone.go` — tone calibration
- Create: `internal/personality/learner.go` — học từ tương tác

**Cơ chế / Mechanism:**
1. Analyze historical conversations → extract tone, humor, formality preferences
2. Adjust system prompt mỗi lần gọi dựa trên learned profile
3. User feedback ("quá formal", "hay đấy") → cập nhật profile
4. Context-dependent: khác khi ở nhà vs khi làm việc

---

## PHASE 14, 21-22: POLISH + APPS

### P14 — Polish + Docs
- README hoàn chỉnh (song ngữ)
- `jarvis --help` có ví dụ cho mọi command
- Install script: `curl -fsSL https://get.jarvis.local | sh`
- Auto-update: `jarvis update`

### P21 — Desktop App (Tauri)
- System tray icon
- Native notifications (macOS/Windows/Linux)
- Global shortcut (Cmd+Shift+J)
- Minimal window: chat input + response

### P22 — Mobile Companion (Optional)
- iOS/Android app kết nối qua local network
- Chỉ hoạt động khi cùng WiFi với máy chạy JARVIS
- Tính năng: voice input, quick questions, push notifications

---

## TỔNG KẾT LỘ TRÌNH / ROADMAP SUMMARY

```
NOW ────── 2 weeks ────── 1 month ────── 3 months ────── 6 months

P2 ████████░░  Agent      JARVIS có thể   JARVIS dùng     JARVIS
   Engine      chat được   được hằng       hằng ngày       hoàn chỉnh
P15 ░░░░░░░░   (Fake LLM)  ngày (Ollama,
P16 ░░░░░░░░               SQLite, tools)
P3 ░░░░░░░░
P17 ░░░░░░░░
P18 ░░░░░░░░
P7-P10 ░░░░░░
P19-P22 ░░░░░

MILESTONES:
────────────
M1 (2 tuần):  Engine chạy với FakeProvider. POST /chat → SSE response.
              → Có thể demo: gõ curl thấy response stream về.

M2 (1 tháng): Ollama local hoạt động. SQLite lưu chat. 5 tools cơ bản.
              → Có thể chat thật với JARVIS trên terminal.

M3 (3 tháng): Đủ tools personal (email, calendar, notes). Memory 4-tier.
              Proactive morning briefing. CLI hoàn chỉnh.
              → JARVIS thay thế được 50% việc hằng ngày.

M4 (6 tháng): Desktop app. Personality engine. Mobile companion.
              → JARVIS hoàn chỉnh như thiết kế.
```
