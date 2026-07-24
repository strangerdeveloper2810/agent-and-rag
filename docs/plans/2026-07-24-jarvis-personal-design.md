# J.A.R.V.I.S. Personal — Design Document
# J.A.R.V.I.S. Cá Nhân — Tài Liệu Thiết Kế

> **Ngày/Date:** 2026-07-24
> **Trạng thái/Status:** Draft v1 — pivot từ `feat/go-agent` sang JARVIS Personal
> **Ngôn ngữ/Language:** Song ngữ Việt-Anh (bilingual). Giải thích bằng tiếng Việt, thuật ngữ kỹ thuật bằng tiếng Anh.
> **Audience:** FE developer learning Go backend + AI agent engineering. Target: interview-ready understanding.

---

## 0. Tầm Nhìn / Vision

### 0.1 JARVIS Personal Là Gì? / What Is JARVIS Personal?

**VI:** JARVIS Personal là một AI assistant (trợ lý AI) chạy **hoàn toàn trên máy cá nhân** (local-first), được thiết kế cho **một người dùng duy nhất**. Nó biết mọi thứ về bạn — lịch sử chat, email, tài liệu, thói quen, sở thích — và tất cả data nằm trong ổ cứng của bạn, không gửi đi đâu cả.

**EN:** JARVIS Personal is a **local-first AI assistant** designed for a **single user**. It knows everything about you — chat history, emails, documents, habits, preferences — and all data stays on your hard drive. Nothing leaves your machine.

### 0.2 Khác Gì Với ChatGPT/Claude? / How Is It Different From ChatGPT/Claude?

| Tiêu chí / Criteria | ChatGPT / Claude | JARVIS Personal |
|---|---|---|
| **Nơi chạy / Where it runs** | Cloud (máy chủ OpenAI/Anthropic) | Máy bạn / Your machine |
| **Biết về bạn / Knows about you** | Chỉ trong 1 phiên chat / Per-session only | Mọi thứ, mãi mãi / Everything, forever |
| **Tools** | Giới hạn / Limited (web search, code) | Mọi thiết bị của bạn / All your devices (MCP) |
| **Privacy** | Data gửi lên cloud | Data không rời máy bạn / Never leaves your machine |
| **Cost** | $20/tháng / $20/month | $0/tháng (sau khi mua phần cứng) / $0/month (after hardware) |
| **Customization** | Prompt + system message | Toàn bộ source code / Full source code |

### 0.3 Nguyên Tắc Thiết Kế / Design Principles

| # | Nguyên tắc / Principle | Giải thích / Explanation |
|---|---|---|
| P1 | **Local-first** | Mọi thứ chạy trên máy bạn. Cloud chỉ là optional fallback. / Everything runs locally. Cloud is optional fallback only. |
| P2 | **Privacy by design** | Data encrypted at rest. Prompt không rời máy (trừ khi bạn chọn cloud LLM). / Data encrypted at rest. Prompts never leave your machine (unless you opt into cloud LLM). |
| P3 | **Deep personalization** | Càng dùng càng thông minh. Học thói quen, giọng văn, sở thích của bạn. / Gets smarter the more you use it. Learns your habits, writing style, preferences. |
| P4 | **Extensible qua MCP** | Mọi thiết bị/phần mềm là 1 MCP server. Bạn tự thêm tool. / Every device/app is an MCP server. You add your own tools. |
| P5 | **Offline-capable** | Mất mạng vẫn hoạt động (với local LLM). / Works without internet (with local LLM). |
| P6 | **Single binary** | `jarvis serve` — 1 câu lệnh, chạy được. / One command, it runs. |

---

## 1. Tổng Quan Kiến Trúc / Architecture Overview

### 1.1 High-Level Architecture / Kiến Trúc Tổng Thể

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        YOUR MACHINE / MÁY CỦA BẠN                        │
│                                                                          │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │                    INTERFACE LAYER / LỚP GIAO DIỆN                │   │
│  │                                                                   │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐         │   │
│  │  │ CLI      │  │ Web UI   │  │ Desktop  │  │ Mobile   │         │   │
│  │  │ (Terminal│  │ (React)  │  │ (Tauri)  │  │ (Companion│        │   │
│  │  │  app)    │  │          │  │          │  │  app)     │         │   │
│  │  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘         │   │
│  │       └──────────────┴─────────────┴─────────────┘               │   │
│  │                          │                                        │   │
│  │               HTTP + SSE (localhost:8080)                          │   │
│  └──────────────────────────┼────────────────────────────────────────┘   │
│                              │                                            │
│  ┌──────────────────────────┼────────────────────────────────────────┐   │
│  │                    CORE ENGINE / ENGINE LÕI                        │   │
│  │                          │                                         │   │
│  │  ┌───────────────────────▼──────────────────────────────────┐     │   │
│  │  │              JARVIS CORE (Go binary, ~30MB)               │     │   │
│  │  │                                                           │     │   │
│  │  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐     │     │   │
│  │  │  │ Agent   │  │ Memory  │  │ Tool    │  │ Context │     │     │   │
│  │  │  │ Engine  │  │ System  │  │ Registry│  │ Engine  │     │     │   │
│  │  │  │ (ReAct) │  │ (4-tier)│  │ (MCP)   │  │         │     │     │   │
│  │  │  └─────────┘  └─────────┘  └─────────┘  └─────────┘     │     │   │
│  │  │                                                           │     │   │
│  │  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐     │     │   │
│  │  │  │ Provider│  │ Guard-  │  │ Person- │  │ Obser-  │     │     │   │
│  │  │  │ Layer   │  │ rails   │  │ ality   │  │ vability│     │     │   │
│  │  │  └─────────┘  └─────────┘  └─────────┘  └─────────┘     │     │   │
│  │  └───────────────────────────────────────────────────────────┘     │   │
│  └────────────────────────────────────────────────────────────────────┘   │
│                                    │                                      │
│  ┌─────────────────────────────────┼──────────────────────────────────┐   │
│  │                      DATA LAYER / LỚP DỮ LIỆU                       │   │
│  │                                 │                                    │   │
│  │  ┌──────────┐  ┌──────────┐  ┌─▼───────┐  ┌──────────┐             │   │
│  │  │ SQLite   │  │ Chroma   │  │ Ollama  │  │ MCP      │             │   │
│  │  │ (chats,  │  │ (vector  │  │ (local  │  │ Servers  │             │   │
│  │  │  memory, │  │  store)  │  │  LLM)   │  │ (devices)│             │   │
│  │  │  config) │  │          │  │         │  │          │             │   │
│  │  └──────────┘  └──────────┘  └─────────┘  └──────────┘             │   │
│  └────────────────────────────────────────────────────────────────────┘   │
│                                                                          │
│  ┌────────────────────────────────────────────────────────────────────┐   │
│  │              OPTIONAL CLOUD FALLBACK / DỰ PHÒNG CLOUD (TÙY CHỌN)   │   │
│  │                                                                     │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐                          │   │
│  │  │ Gemini   │  │ Claude   │  │ GitHub   │  (dùng API key của bạn)   │   │
│  │  │ API      │  │ API      │  │ Backup   │  (using YOUR API keys)    │   │
│  │  └──────────┘  └──────────┘  └──────────┘                          │   │
│  └────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────┘
```

### 1.2 Stack So Sánh Với Hiện Tại / Stack Comparison With Current Project

| Layer | Current Project (learning) | JARVIS Personal |
|---|---|---|
| **Language** | Go 1.24 (agent) + TypeScript (gateway) | **Go only** — single binary, no gateway needed |
| **Database** | MongoDB Atlas (cloud) | **SQLite** (local) + **Chroma** (embedded vector) |
| **LLM** | Gemini/Claude API (cloud) | **Ollama** (local Llama/Mistral) → cloud fallback |
| **Embedding** | Voyage AI API (cloud) | **Ollama embedding** (local nomic-embed-text) |
| **Transport** | HTTP + SSE (api → agent-go) | **HTTP + SSE** (UI → core, direct) |
| **Deploy** | Docker Compose (api + agent-go + web) | **Single binary** `jarvis` |
| **Gateway** | Fastify/TS (apps/api) | **Không cần** — Go serve trực tiếp |
| **PDF/File** | TS (pdf-parse, mammoth) | **Go** (ledongthuc/pdf, or exec pdftotext) |
| **Auth** | Chưa có / Not yet | **Không cần** — single user, localhost |
| **Billing** | Không / None | **Không cần** — your hardware, your rules |

### 1.3 What We Keep From Current Project / Những Gì Giữ Lại

Toàn bộ engine hiện tại được GIỮ NGUYÊN — chỉ đổi infrastructure layer:

| Component | Keep? | Ghi chú / Note |
|---|---|---|
| `internal/agent/` (engine, state, router, nodes) | ✅ Keep | Core ReAct loop không đổi |
| `internal/provider/` (Gemini, Claude, Fake) | ✅ Keep + Add | Thêm Ollama adapter |
| `internal/tools/` (Registry, Tool interface) | ✅ Keep | Mở rộng tool definitions |
| `internal/memory/` (memory system) | ✅ Keep | Đổi backend từ Mongo → SQLite |
| `internal/guardrails/` | ✅ Keep | Đơn giản hóa (single user) |
| `internal/rag/` | ✅ Keep | Đổi Voyage → local embedding |
| `internal/mongo/` | ❌ Replace | SQLite + Chroma |
| `internal/config/` | ✅ Keep + Modify | Thêm local paths, device configs |
| `apps/api/` (TS gateway) | ❌ Remove | Go serve trực tiếp |
| `apps/web/` (React frontend) | ✅ Keep | Cải tiến UI cho personal use |

---

## 2. Core Engine / Engine Lõi

### 2.1 Agent Loop (ReAct) — Giữ Nguyên Từ P2

```
Giống hệt engine hiện tại / Same as current engine:

┌──────────┐     ┌──────────┐     ┌──────────┐     ┌──────────┐
│  RECALL  │────►│  MODEL   │────►│  ROUTE   │────►│  TOOLS   │
│ (memory) │     │ (LLM)    │     │ (decide) │     │ (execute)│
└──────────┘     └──────────┘     └──────────┘     └────┬─────┘
      ▲                                                 │
      │                                          ┌──────▼─────┐
      └──────────────────────────────────────────│   MODEL    │
                                                  │ (see tools │
                                                  │  results)  │
                                                  └────────────┘
```

**Khác biệt duy nhất / Only difference:** LLM call đi qua Ollama local trước, cloud fallback sau.

### 2.2 State Machine — Thêm Trường Cho Personal Context

```go
// State — giữ nguyên từ P2 + thêm personal context
type State struct {
    // --- Giữ nguyên từ P2 (Keep from P2) ---
    Messages    []provider.Message  // conversation history
    Scratchpad  []Observation       // tool execution results
    Step        int                 // current loop iteration
    MaxSteps    int                 // safety limit (default: 12)
    Usage       provider.Usage      // token accounting
    Done        bool                // termination flag
    Interrupt   *Interrupt          // HITL pause marker (rarely needed)

    // --- THÊM MỚI cho Personal (New for Personal) ---
    UserContext *UserContext        // thông tin người dùng hiện tại
    MemoryTier  []MemoryRecall      // kết quả recall từ memory system
}

// UserContext — ngữ cảnh người dùng (biết bạn đang ở đâu, làm gì)
type UserContext struct {
    TimeOfDay    string    // "morning", "afternoon", "evening", "night"
    DayOfWeek    string    // "monday", "saturday"...
    Location     string    // "home", "office", "traveling" (từ WiFi/calendar)
    Activity     string    // "working", "coding", "relaxing", "commuting"
    DeviceType   string    // "desktop", "laptop", "phone"
    FocusMode    bool      // bạn có đang focus không? → giảm interruption
    Mood         string    // (optional) từ tone của tin nhắn hoặc health data
}
```

---

## 3. LLM Layer / Lớp Mô Hình Ngôn Ngữ

### 3.1 Provider Chain / Chuỗi Provider

```
User request
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│                    PROVIDER CHAIN                            │
│                                                              │
│  1. OLLAMA LOCAL (primary)                                   │
│     ├─ Model: llama3.1:70b / mistral-large / qwen2.5:72b   │
│     ├─ Latency: 50-200ms (RTX 4090)                          │
│     ├─ Cost: $0                                              │
│     ├─ Privacy: ✅ data không rời máy                        │
│     └─ Fail? → fallback to #2                                │
│                                                              │
│  2. CLOUD API (fallback)                                     │
│     ├─ Gemini 3.1 Flash Lite (rẻ nhất, nhanh nhất)          │
│     ├─ Claude Haiku (tốt hơn, đắt hơn)                       │
│     ├─ Claude Opus (mạnh nhất, đắt nhất) — dùng cho task khó│
│     └─ Dùng API key CỦA BẠN (bạn trả tiền, bạn kiểm soát)  │
│                                                              │
│  3. TIERED ROUTING (cost optimization)                       │
│     ├─ Task đơn giản (chat, summarize) → local Llama        │
│     ├─ Task trung bình (code review, translation) → Gemini  │
│     └─ Task phức tạp (debug, architecture) → Claude Opus    │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 Ollama Adapter — Thêm Vào Provider Layer

```go
// internal/provider/ollama/ollama.go — adapter MỚI
type Client struct {
    baseURL    string           // http://localhost:11434
    model      string           // "llama3.1:70b"
    httpClient *http.Client
}

func (c *Client) Generate(ctx context.Context, req provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
    // Gọi Ollama REST API:
    // POST http://localhost:11434/api/chat
    // {
    //   "model": "llama3.1:70b",
    //   "messages": [...],
    //   "tools": [...],
    //   "stream": true
    // }
    //
    // Response: từng dòng JSON { "message": {"content": "..."}, "done": false }
    // Map về provider.StreamChunk giống hệt Gemini/Claude adapter
}

// Embedding adapter — thay thế Voyage
func (c *Client) Embed(ctx context.Context, texts []string) ([][]float32, error) {
    // POST http://localhost:11434/api/embed
    // { "model": "nomic-embed-text", "input": texts }
}
```

### 3.3 Tiered Router / Định Tuyến Theo Độ Phức Tạp

```go
// internal/provider/tiered/tiered.go
type TieredProvider struct {
    local     provider.Provider  // Ollama
    cheap     provider.Provider  // Gemini Flash Lite
    strong    provider.Provider  // Claude Opus / GPT-5
    classifier *TaskClassifier  // model nhỏ (7B) phân loại task
}

func (t *TieredProvider) Generate(ctx context.Context, req provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
    tier := t.classifier.Classify(req)  // → TierSimple | TierMedium | TierComplex

    switch tier {
    case TierSimple:
        if t.local.IsHealthy() {
            return t.local.Generate(ctx, req)  // local Llama
        }
        return t.cheap.Generate(ctx, req)      // fallback Gemini
    case TierMedium:
        return t.cheap.Generate(ctx, req)      // Gemini Flash Lite
    case TierComplex:
        return t.strong.Generate(ctx, req)     // Claude Opus
    }
}
```

---

## 4. Memory System / Hệ Thống Bộ Nhớ (4-Tier)

### 4.1 Kiến Trúc 4 Tầng / 4-Tier Architecture

```
┌──────────────────────────────────────────────────────────────────────┐
│ TIER 1: WORKING MEMORY (bộ nhớ làm việc)                             │
│ ─────────────────────────────────────────────────────────────────── │
│ Scope:      1 phiên chat hiện tại / current conversation             │
│ Storage:    RAM (State.Messages)                                     │
│ Capacity:   128K tokens (context window)                             │
│ Lifecycle:  Tạo mới mỗi request, xóa sau khi response               │
│ Purpose:    LLM thấy toàn bộ context của cuộc trò chuyện hiện tại   │
│                                                                      │
│ Ví dụ: "JARVIS, phân tích báo cáo này" → context có:                 │
│   - 5 tin nhắn gần nhất                                              │
│   - Kết quả tool (nội dung báo cáo)                                  │
│   - Memory recall (sở thích của bạn về format báo cáo)               │
├──────────────────────────────────────────────────────────────────────┤
│ TIER 2: EPISODIC MEMORY (bộ nhớ tình tiết)                           │
│ ─────────────────────────────────────────────────────────────────── │
│ Scope:      Tất cả cuộc trò chuyện / all conversations               │
│ Storage:    SQLite table `episodes`                                   │
│ Capacity:   Unlimited (chỉ giới hạn bởi ổ cứng)                      │
│ Retrieval:  Vector similarity (Chroma) + full-text search (SQLite)   │
│ Purpose:    "JARVIS, nhớ cái task hôm qua tôi giao không?"           │
│                                                                      │
│ Schema:                                                              │
│   id, timestamp, summary (text), embedding (vector),                 │
│   conversation_id, key_moments (JSON), tags                           │
├──────────────────────────────────────────────────────────────────────┤
│ TIER 3: SEMANTIC MEMORY (bộ nhớ ngữ nghĩa — vĩnh viễn)              │
│ ─────────────────────────────────────────────────────────────────── │
│ Scope:      Tất cả kiến thức về bạn / all knowledge about you        │
│ Storage:    SQLite table `memories` + Chroma vector index            │
│ Capacity:   Unlimited                                                │
│ Retrieval:  Structured (type+key) + Vector (cosine similarity)       │
│ Purpose:    "JARVIS, tôi thích uống cà phê loại gì?"                 │
│                                                                      │
│ Schema:                                                              │
│   id, type (preference|fact|entity|relationship),                     │
│   key, value, confidence (0.0-1.0), source (ai_extracted|manual),    │
│   embedding (1024d), created_at, updated_at, access_count            │
├──────────────────────────────────────────────────────────────────────┤
│ TIER 4: PROCEDURAL MEMORY (bộ nhớ quy trình)                         │
│ ─────────────────────────────────────────────────────────────────── │
│ Scope:      Các pattern, workflow, cách làm đã được học              │
│ Storage:    SQLite table `procedures` + file system (skills/)        │
│ Purpose:    "JARVIS, làm như lần trước đi"                           │
│                                                                      │
│ Schema:                                                              │
│   id, name, description, trigger_keywords (JSON),                    │
│   steps (JSON), success_rate (0.0-1.0), last_used, use_count         │
│                                                                      │
│ Ví dụ:                                                               │
│   {                                                                  │
│     "name": "daily_standup_prep",                                    │
│     "trigger": ["chuẩn bị standup", "daily update"],                │
│     "steps": [                                                       │
│       "1. Check git commits since yesterday",                        │
│       "2. Check Jira tickets updated",                               │
│       "3. Check calendar for today's meetings",                      │
│       "4. Format as bullet-point summary"                            │
│     ],                                                               │
│     "success_rate": 0.95                                             │
│   }                                                                  │
└──────────────────────────────────────────────────────────────────────┘
```

### 4.2 Memory Flow / Luồng Hoạt Động

```go
// 1. RECALL — trước mỗi lần gọi model
func (m *MemorySystem) Recall(ctx context.Context, query string, userCtx *UserContext) *MemoryContext {
    // Structured lookup: type+key exact match (nhanh nhất)
    facts := m.sqlite.LookupFacts(userCtx.Preferences) // "coffee_preference"

    // Vector recall: semantic similarity (sâu hơn)
    embedding := m.ollama.Embed(ctx, query)
    related := m.chroma.Search(ctx, embedding, TopK=10)

    // Episodic recall: "có cuộc trò chuyện nào liên quan không?"
    episodes := m.chroma.SearchEpisodes(ctx, embedding, TopK=3)

    // Procedural match: "có workflow nào cho việc này không?"
    procedures := m.sqlite.MatchProcedures(query)

    // MERGE + DEDUP + RANK
    return m.merge(facts, related, episodes, procedures)
}

// 2. EXTRACT — sau mỗi cuộc trò chuyện
func (m *MemorySystem) Extract(ctx context.Context, conversation []provider.Message) {
    // Gọi LLM (local, rẻ): "Trích xuất facts và preferences mới từ cuộc trò chuyện này"
    facts := m.llm.ExtractFacts(ctx, conversation)
    // → "user thích cafe đen không đường"
    // → "user đang làm dự án X, deadline 2026-08-15"
    // → "user không thích họp sáng thứ 2"

    // UPSERT: ghi đè nếu type+key trùng, giữ confidence cao nhất
    for _, fact := range facts {
        m.sqlite.UpsertMemory(fact)
        m.chroma.UpsertEmbedding(fact)
    }
}

// 3. SUMMARIZE — khi context window sắp đầy
func (m *MemorySystem) Summarize(ctx context.Context, messages []provider.Message) string {
    // Khi history > 20 messages → nén thành summary
    // Giữ 10 message gần nhất + 1 summary của phần cũ
    return m.llm.Summarize(ctx, messages[:len(messages)-10])
}
```

---

## 5. Tool System / Hệ Thống Công Cụ (MCP)

### 5.1 Tool Categories / Phân Loại Tool

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        JARVIS PERSONAL TOOLS                             │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  SYSTEM TOOLS (mặc định, luôn có)                                        │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │ file.search    │ Tìm file trên máy bạn (grep, glob)               │   │
│  │ file.read      │ Đọc file (text, code, config)                    │   │
│  │ file.write     │ Ghi file (code generation, note taking)          │   │
│  │ shell.exec     │ Chạy lệnh terminal (có xác nhận cho destructive) │   │
│  │ web.search     │ Tìm kiếm web (qua SearXNG local hoặc API)       │   │
│  │ web.fetch      │ Đọc nội dung trang web                           │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                          │
│  PERSONAL TOOLS (kết nối với data của bạn)                               │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │ email.search   │ Tìm email (IMAP local index)                     │   │
│  │ email.send     │ Gửi email (SMTP)                                 │   │
│  │ calendar.list  │ Xem lịch (iCal/Google Calendar local sync)       │   │
│  │ calendar.create│ Tạo sự kiện                                      │   │
│  │ contacts.find  │ Tìm liên hệ                                      │   │
│  │ notes.search   │ Tìm ghi chú (Obsidian/Apple Notes local)         │   │
│  │ notes.create   │ Tạo ghi chú mới                                  │   │
│  │ browser.history│ Tìm trong lịch sử duyệt web                      │   │
│  │ health.latest  │ Dữ liệu sức khỏe mới nhất (Apple Health sync)    │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                          │
│  DEV TOOLS (nếu bạn là developer)                                        │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │ git.log        │ Xem git history                                  │   │
│  │ git.diff       │ Xem thay đổi                                     │   │
│  │ github.pr      │ Quản lý pull requests                            │   │
│  │ jira.tickets   │ Xem Jira tickets                                 │   │
│  │ db.query       │ Query database (read-only by default)             │   │
│  │ code.explain   │ Giải thích code                                  │   │
│  │ code.review    │ Review pull request                               │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                          │
│  SMART HOME TOOLS (nếu có thiết bị thông minh)                           │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │ home.lights    │ Điều khiển đèn                                   │   │
│  │ home.climate   │ Điều chỉnh nhiệt độ                              │   │
│  │ home.security  │ Kiểm tra camera                                  │   │
│  │ home.music     │ Phát nhạc (Spotify/YouTube Music API)            │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                          │
│  CUSTOM TOOLS (bạn tự code)                                              │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │ ~/.jarvis/tools/your-tool.yaml  → JARVIS tự load                  │   │
│  │ Hoặc MCP server chạy local → JARVIS auto-discover                │   │
│  └──────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────┘
```

### 5.2 MCP Architecture / Kiến Trúc MCP

```go
// JARVIS auto-discovers local MCP servers
type MCPRegistry struct {
    servers map[string]*MCPClient  // tool_name → client connection
}

func (r *MCPRegistry) Discover(ctx context.Context) error {
    // 1. Đọc config: ~/.jarvis/mcp-servers/*.yaml
    // 2. Kết nối đến từng MCP server qua stdio hoặc localhost
    // 3. Gọi tools/list → lấy danh sách tool
    // 4. Đăng ký vào Tool Registry

    for _, cfg := range r.loadConfigs("~/.jarvis/mcp-servers/") {
        client, err := mcp.Connect(ctx, cfg.Command, cfg.Args)
        tools, err := client.ListTools(ctx)
        for _, tool := range tools {
            r.registry.Register(adaptMCPTool(tool, client))
        }
    }
    return nil
}
```

---

## 6. Interface Layer / Lớp Giao Diện

### 6.1 CLI First — Tương Tác Bằng Dòng Lệnh

```bash
# Cách dùng cơ bản
$ jarvis "What's on my calendar today?"
$ jarvis "Tóm tắt 3 email gần nhất"
$ jarvis "Tạo task review PR #42 trước 5h chiều"

# Interactive mode (chat liên tục)
$ jarvis chat
JARVIS> Hello Tony. You have 3 meetings today. Coffee?
You>    No coffee today, I'm trying to cut down
JARVIS> Noted. I've updated your preferences. Shall I order green tea instead?

# Pipe mode (dùng với tool khác)
$ cat error.log | jarvis "Analyze this error log"
$ git diff HEAD~5 | jarvis "Summarize these changes for standup"

# Background mode (JARVIS chạy nền, proactive)
$ jarvis serve
# → HTTP server at localhost:8080
# → WebSocket for real-time events
# → Proactive watchers running
```

### 6.2 Web UI — React App (Giữ Từ apps/web)

```
┌──────────────────────────────────────────────────────────┐
│  JARVIS                                  [Settings] [⚡] │
├──────────────────────────────────────────────────────────┤
│                                                          │
│  ┌──────────────────────────────────────────────────┐   │
│  │ Today, 09:30                                     │   │
│  │                                                  │   │
│  │ JARVIS: Good morning. Here's your briefing:      │   │
│  │                                                  │   │
│  │ 📅 10:00 — Standup meeting                       │   │
│  │ 📧 3 unread emails from Pepper, Happy, Rhodey    │   │
│  │ 🔧 PR #42 needs review (requested by Bruce)      │   │
│  │ 💡 You left off working on arc-reactor.cad       │   │
│  │                                                  │   │
│  │ Would you like me to:                            │   │
│  │ [Prepare standup update] [Review PR] [Continue CAD]│
│  └──────────────────────────────────────────────────┘   │
│                                                          │
│  ┌──────────────────────────────────────────────────┐   │
│  │ Type a command or message...                      │   │
│  └──────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────┘
```

### 6.3 Proactive Notifications / Thông Báo Chủ Động

```go
// JARVIS không chỉ phản hồi — nó CHỦ ĐỘNG thông báo
type ProactiveEngine struct {
    watchers []ScheduledWatcher
    triggers []EventTrigger
}

// Scheduled: chạy theo lịch
var morningBriefing = ScheduledWatcher{
    Cron:    "0 7 * * *",           // 7:00 AM mỗi ngày
    Action:  "generateMorningBriefing",
}

// Event-driven: chạy khi có sự kiện
var emailFromBoss = EventTrigger{
    Condition: "new_email AND sender IN [boss_list]",
    Action:    "priority_notification",
}
```

---

## 7. Privacy & Security / Bảo Mật & Riêng Tư

### 7.1 Data Flow / Luồng Dữ Liệu

```
LOCAL (máy bạn)                           CLOUD (tùy chọn)
─────────────────────                     ─────────────────
                                            
Prompt → JARVIS Core ──► [local LLM]      (Prompt không rời máy)
                            │                
                            │ fail?          
                            ▼                
                        [cloud LLM] ←──► Prompt gửi lên API (dùng key của bạn)
                                            
SQLite DB ←── Mọi data cá nhân             (Không ai thấy ngoài bạn)
                                            
Chroma DB ←── Embeddings                   (Local embedding model)
```

### 7.2 Encryption / Mã Hóa

```go
// Data at rest: SQLite với SQLCipher (AES-256)
// Data in transit (cloud fallback): TLS 1.3
// Memory: Go's memory safety (no buffer overflow)

type SecureStore struct {
    db *sqlite.DB  // SQLCipher encrypted
}

func (s *SecureStore) Open(path, passphrase string) error {
    // PRAGMA key = 'your-passphrase';
    // PRAGMA cipher_page_size = 4096;
    // PRAGMA kdf_iter = 256000;
}
```

---

## 8. Roadmap / Lộ Trình

### Phase Map: Từ feat/go-agent → JARVIS Personal

```
HIỆN TẠI (feat/go-agent)           JARVIS PERSONAL
─────────────────────────          ────────────────────

P0: Scaffold Go ✅                  → Giữ nguyên
P1: Provider (Gemini+Claude) ✅     → + Thêm Ollama adapter
P2: Agent Engine 🔄                 → Giữ nguyên, thêm UserContext
P3: Tool System 📋                  → + MCP discovery + personal tools
P4: MongoDB 📋                      → THAY BẰNG SQLite + Chroma
P5: RAG (Voyage) 📋                 → THAY BẰNG local embedding
P6: Context Engineering 📋          → Giữ nguyên, thêm personal context
P7: Memory 3-tier 📋                → NÂNG CẤP lên 4-tier + procedural
P8: Planner + Reflection 📋         → Giữ nguyên
P9: Skills 📋                       → + Custom skill loader (~/.jarvis/skills)
P10: Guardrails + HITL 📋           → Đơn giản hóa (single user)
P11: Observability 📋               → Giữ nguyên (local OTel)
P12: Gateway Integration 📋         → BỎ (không cần gateway)
P13: Eval 📋                        → Giữ nguyên
P14: Polish 📋                       → + CLI, Desktop app, installer

MỚI:
P15: Ollama Integration              → Local LLM adapter
P16: SQLite Migration                → Chuyển từ MongoDB → SQLite
P17: MCP Device Layer                → Auto-discover + device tools
P18: CLI Interface                   → Terminal-first interaction
P19: Proactive Engine                → Scheduled + event-driven watchers
P20: Personality Engine              → Learn & adapt to user's style
P21: Desktop App (Tauri)             → System tray + native notifications
P22: Mobile Companion                → Optional: iOS/Android via local network
```

---

## 9. File Structure / Cấu Trúc File

```
jarvis/                              # ~/jarvis — thư mục gốc của JARVIS
├── jarvis                           # Single binary (Go build)
├── config.yaml                      # ~/.jarvis/config.yaml
├── data/
│   ├── jarvis.db                    # SQLite (chats, memory, config)
│   └── chroma/                      # Chroma vector store
├── mcp-servers/                     # MCP server configs
│   ├── email.yaml
│   ├── calendar.yaml
│   └── github.yaml
├── skills/                          # Custom skills (SKILL.md)
│   ├── standup-prep/
│   │   └── SKILL.md
│   └── code-review/
│       └── SKILL.md
├── logs/                            # Application logs
│   └── jarvis.log
└── backups/                         # Auto-backup (daily)
    └── jarvis-2026-07-24.db.gz

Source code (monorepo):
services/jarvis/                     # ← RENAME từ services/agent-go
├── cmd/jarvis/main.go               # Entry point: CLI + server
├── internal/
│   ├── agent/                       # Engine (giữ từ P2)
│   ├── provider/                    # + ollama/
│   ├── tools/                       # + mcp/
│   ├── memory/                      # 4-tier system
│   ├── storage/                     # SQLite + Chroma (thay mongo/)
│   ├── personality/                 # Personality engine (mới)
│   ├── proactive/                   # Proactive watchers (mới)
│   └── ...
├── skills/                          # Bundled skills
├── web/                             # React UI (apps/web → move vào đây)
└── installer/                       # Install script (brew, curl|sh)
```

---

## 10. So Sánh Với Hiện Tại / Comparison With Current Project

| Khía cạnh / Aspect | Current `feat/go-agent` | JARVIS Personal |
|---|---|---|
| **Mục tiêu / Goal** | Học agent engineering | Build AI assistant XÀI THẬT |
| **Users** | 1 (bạn, developer) | 1 (bạn, hoặc gia đình) |
| **Database** | MongoDB Atlas (cloud, free tier) | SQLite + Chroma (local, unlimited) |
| **LLM** | Gemini/Claude API | Ollama local → cloud fallback |
| **Embedding** | Voyage AI (cloud) | nomic-embed-text (local) |
| **Gateway** | Fastify/TS (apps/api) | Không cần — Go serve direct |
| **Tools** | 9 tools (RAG + tasks) | 30+ tools (system + personal + devices) |
| **Memory** | 3-tier (in progress) | 4-tier (thêm procedural) |
| **Interface** | Browser (localhost:5173) | CLI + Web UI + Desktop app |
| **Deploy** | Docker compose (3 services) | Single binary |
| **Config** | Env vars (`.env`) | YAML file (`~/.jarvis/config.yaml`) |
| **Personality** | System prompt cố định | Learns & adapts |
| **Proactive** | Không (reactive only) | Có (scheduled + event-driven) |
| **Offline** | Không (cần cloud LLM + Mongo) | Có (local LLM + SQLite) |
