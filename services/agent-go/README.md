# agent-go — JARVIS Agent Runtime

Agent runtime viết bằng **Go** — trái tim của JARVIS platform. Tự xây từ đầu (không LangGraph): state machine, ReAct loop, multi-agent orchestrator, tool registry (25 tools), 3-tier memory + autonomous learner, MCP client, personality engine, proactive scheduler, và SSE streaming.

Polyglot với `apps/api` Fastify/TypeScript (gateway) và `apps/web` React (frontend).

---

## Quick Links

| Document | Description |
|----------|-------------|
| [README.md](../../README.md) | Tổng quan dự án JARVIS (bilingual VI-EN) |
| [docs/API.md](../../docs/API.md) | API reference: POST /chat (SSE), GET /healthz |
| [docs/TOOLS.md](../../docs/TOOLS.md) | Tool development guide + built-in tools reference |
| [docs/DEPLOY.md](../../docs/DEPLOY.md) | Deployment: Docker, Docker Compose, env vars |
| [docs/SKILLS.md](../../docs/SKILLS.md) | Skills system (progressive disclosure) |
| [docs/DEVELOPMENT.md](../../docs/DEVELOPMENT.md) | Dev guide: add provider, add agent, code style |
| [docs/architecture/](../../docs/architecture/) | Architecture deep-dives + system design |

---

## Structure

```
services/agent-go/
├── cmd/
│   ├── server/main.go       # HTTP server entrypoint (SSE chat + health + suggestions)
│   └── jarvis/main.go       # CLI entrypoint (serve / ask / chat REPL)
├── internal/
│   ├── agent/               # Core engine: State, Nodes (plan/model/route/tools/extract), Router, Events, ReAct loop
│   ├── orchestrator/        # Multi-agent orchestration: IntentRouter (keyword) chọn agent, HandoffManager (agent-to-agent)
│   ├── provider/            # LLM abstraction
│   │   ├── factory/         # Chọn provider theo LLM_PROVIDER (gemini/anthropic/deepseek/auto)
│   │   ├── fallback/        # Auto-fallback chain: DeepSeek → Gemini → Claude, cooldown per-provider
│   │   ├── gemini/          # Gemini adapter (genai SDK) + prompt caching
│   │   ├── anthropic/       # Claude adapter (anthropic-sdk-go) + prompt caching
│   │   ├── deepseek/        # DeepSeek adapter (OpenAI-compatible), auto-route flash/pro theo độ phức tạp
│   │   └── ollama/          # Local Ollama adapter (dev/offline)
│   ├── tools/                # Tool interface, registry, parallel fan-out — 25 tools (xem bảng dưới)
│   ├── memory/               # 3-tier: working (in-msg), episodic (summarize), semantic (extract+store) + autonomous learner
│   ├── personality/          # Personality profile (formality, humor, verbosity) — AdaptPrompt + Learn theo thời gian
│   ├── proactive/             # Cron scheduler (robfig/cron) — gọi prompt định kỳ, tích hợp với engine
│   ├── mcp/                   # MCP client qua subprocess stdin/stdout JSON-RPC 2.0 + auto-discovery tool từ YAML
│   ├── guardrails/            # Circuit breaker (chống stuck loop), tool guard (Read/Write/Destructive + HITL), prompt-injection filter
│   ├── mongo/                  # MongoDB driver (tasks, documents, memories)
│   ├── storage/
│   │   ├── sqlite/            # SQLite (modernc.org/sqlite, pure Go) — conversations/messages/memories offline
│   │   └── chroma/             # In-memory vector store (cosine similarity) — MVP semantic search
│   ├── rag/                    # Voyage AI embedding + Atlas vector search (Parent Document Retrieval, HyDE, LLM rerank)
│   ├── skills/                 # Progressive disclosure engine: ListSkills / LoadSkill / MatchSkill (30 SKILL.md)
│   ├── eval/                   # EvalHarness (exact/contains/regex/LLM-judge) — thư viện, chưa wire vào CLI
│   ├── metrics/                # Snapshot metrics: requests, tokens, latency, tool calls, errors (thread-safe)
│   ├── config/                 # Env-based configuration (fail-fast)
│   ├── middleware/              # Context key tenant ID, CORS middleware (dev, khi gọi trực tiếp không qua gateway)
│   ├── transport/http/          # SSE chat handler, health/readyz, suggestions endpoint
│   └── observability/           # slog + OpenTelemetry (tracer còn noop, chưa có exporter thật)
├── skills/                     # 30 SKILL.md files (progressive disclosure data)
├── eval/                       # (thư mục riêng cho eval dataset — xem internal/eval để biết harness)
├── go.mod / go.sum
└── Dockerfile                  # Multi-stage distroless (~15MB image)
```

---

## Quick Start

```bash
# Set env (xem env/.env.example ở repo root để biết đầy đủ biến)
export MONGODB_URI="mongodb+srv://..."
export LLM_PROVIDER="auto"        # gemini | anthropic | deepseek | auto (fallback chain)
export GEMINI_API_KEY="your-key"
export DEEPSEEK_API_KEY="your-key"   # tuỳ chọn — immediate cheap fallback
export ANTHROPIC_API_KEY="your-key"  # tuỳ chọn — last-resort fallback

# HTTP server
go run ./cmd/server

# CLI: one-shot question
go run ./cmd/jarvis ask "Thời tiết hôm nay thế nào?"

# CLI: interactive chat (REPL)
go run ./cmd/jarvis chat
```

### Biến env quan trọng

| Biến | Mặc định | Mô tả |
|------|---------|-------|
| `LLM_PROVIDER` | `gemini` | `gemini` \| `anthropic` \| `deepseek` \| `auto` (fallback: DeepSeek → Gemini → Claude) |
| `ENABLE_PLANNING` | `false` | Bật node plan/reflect cho request phức tạp (tốn thêm 1 LLM call trước token đầu) |
| `ENABLE_LEARNER` | `false` | Bật autonomous learner — chạy nền trích xuất user facts + knowledge từ hội thoại |
| `ALLOW_DESTRUCTIVE_TOOLS` | `false` | Cho phép agent tự chạy tool destructive (`shell.exec`) không cần xác nhận HITL |
| `OWNER_TENANT_IDS` | (rỗng) | Danh sách tenant được dùng nhóm tool đặc quyền (`file.read/write/search`, `shell.exec`, `git`) — rỗng = fail-closed, chỉ tenant `default` (local) dùng được |
| `MAX_OUTPUT_TOKENS` | `8192` | Trần output token mỗi lần gọi LLM; `0` = không giới hạn |

---

## API

| Method | Path | Description |
|--------|------|--------------|
| `POST` | `/chat` | Send message, receive SSE stream of agent events |
| `GET` | `/healthz` | Liveness check (`{"status":"ok"}`) |
| `GET` | `/readyz` | Readiness check (provider + Mongo ping) |
| `GET` | `/suggestions` | Gợi ý câu hỏi/prompt (dùng cho frontend, thường qua CORS dev-only) |

See [docs/API.md](../../docs/API.md) for full request/response format and all SSE event types.

---

## CLI

```bash
# Start HTTP server
jarvis serve

# One-shot Q&A
jarvis ask "câu hỏi của bạn"

# Interactive REPL
jarvis chat
> Xin chào
> /exit    # thoát
```

---

## Tools (25)

| Nhóm | Tool | Mô tả ngắn |
|------|------|-----------|
| Utility | `calculator`, `datetime`, `timer`, `json`, `translate`, `echo`, `version` | Tính toán, thời gian, timer, xử lý JSON, dịch, echo test, phiên bản |
| File (đặc quyền — chỉ owner tenant) | `file.search`, `file.read`, `file.write` | Đọc/ghi/tìm file — giới hạn đường dẫn (`AllowedPaths`) |
| Shell/Git (đặc quyền) | `shell.exec`, `git` | Chạy lệnh shell (destructive, cần HITL hoặc `ALLOW_DESTRUCTIVE_TOOLS`), thao tác git |
| Web | `web.search`, `web.fetch`, `http` | Tìm kiếm DuckDuckGo, fetch HTML→text, gọi HTTP tuỳ ý |
| Memory | `memory.save`, `memory.recall`, `memory.list` | Ghi/truy hồi/liệt kê bộ nhớ ngữ nghĩa dài hạn |
| Notes | `notes.search`, `notes.create` | Ghi chú cá nhân (tenant-scoped) |
| RAG | `rag.search`, `rag.list`, `rag.read` | Tìm kiếm vector, liệt kê ĐẦY ĐỦ tài liệu KB, đọc nội dung 1 tài liệu |
| Khác | `calendar`, `weather` | Lịch, thời tiết |

Nhóm "đặc quyền" chỉ khả dụng cho tenant nằm trong `OWNER_TENANT_IDS` (xem [`internal/tools/privileged.go`](internal/tools/privileged.go) và [`internal/guardrails`](internal/guardrails)) — vì các tool này tác động lên MÁY CHẠY AGENT, không scope theo tenant.

---

## Testing

```bash
go test ./...                    # all tests (79 file _test.go)
go test -v ./internal/agent/...  # engine tests with FakeProvider
go test -v ./internal/tools/...  # tool execution + concurrency
go test -race ./...              # race detection
go test -cover ./...             # coverage
```

---

## Design Decisions

| Decision | Why |
|----------|-----|
| **Self-built engine** (no LangGraph) | Full control, learning, zero dependency on Python/TS framework |
| **SSE, not WebSocket** | Unidirectional server->client, browser-native EventSource, simpler |
| **Stateless agent** | History sent per request — no sticky sessions, horizontal scaling |
| **Pluggable providers** | `Provider` interface — swap LLM via env var, zero engine changes |
| **Auto-fallback chain (DeepSeek → Gemini → Claude)** | DeepSeek ~10x rẻ hơn Gemini, ~50x rẻ hơn Claude — fallback rate-limit với zero cooldown, không làm gián đoạn trải nghiệm |
| **FakeProvider** for testing | Deterministic, no network, tests run in ms |
| **errgroup/goroutine** for tool fan-out | `WaitGroup` + error propagation, cleaner API than bare goroutines; timeout mặc định 60s cho mỗi tool |
| **json.RawMessage** for schema/args | Deferred parsing, zero-allocation passthrough to LLM |
| **SQLite + in-memory Chroma-style store** | Chạy được offline/local, không phụ thuộc Mongo Atlas cho dev nhanh; Mongo vẫn là nguồn thật cho production |
| **MCP qua subprocess JSON-RPC** | Mở rộng tool set qua process ngoài (theo chuẩn Model Context Protocol) mà không cần build lại binary |
| **Owner-tenant gating cho tool đặc quyền** | `file.*`/`shell.exec`/`git` tác động lên máy chủ, không scope được theo tenant → mặc định fail-closed, chỉ bật cho tenant khai báo trong `OWNER_TENANT_IDS` |
| **Multi-stage Docker + distroless** | ~15MB image, no shell, minimal attack surface |

---

## Architecture

```
POST /chat (JSON)
      │
      ▼
┌─────────────────────┐
│   Orchestrator       │  IntentRouter (keyword): "code", "debug" → code agent
│   (multi-agent)      │  HandoffManager: agent-to-agent delegation
└────────┬─────────────┘
         │
         ▼
┌─────────────────────┐
│   Engine (ReAct)      │  recall → summarize → model → route → tools → extract
│                       │      ↑                                  │
│  (+ plan/reflect nếu  │      └──────── tools → model ────────────┘
│   ENABLE_PLANNING)    │
└────┬─────────────┬────┘
     │              │
     ▼              ▼
┌──────────┐   ┌───────────┐   ┌──────────────┐   ┌─────────────┐
│ Provider │   │  Tools     │   │  Guardrails  │   │   Memory     │
│ Gemini   │   │  Registry  │   │  circuit     │   │  3-tier +    │
│ DeepSeek │   │  (25, chạy │   │  breaker,    │   │  learner     │
│ Claude   │   │  song song)│   │  tool guard, │   │  (SQLite/    │
│ (auto-   │   │            │   │  HITL,       │   │  Mongo)      │
│ fallback)│   │            │   │  anti-inject │   │              │
└──────────┘   └───────────┘   └──────────────┘   └─────────────┘

Bên cạnh đó: Personality (adapt giọng điệu), Proactive (cron prompt định kỳ),
MCP client (tool ngoài qua JSON-RPC), Skills (30 SKILL.md, progressive disclosure).
```
