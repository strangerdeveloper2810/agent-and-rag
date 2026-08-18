# agent-go — JARVIS Agent Runtime

Agent runtime viet bang **Go** — trai tim cua JARVIS platform. Tu xay tu dau (khong LangGraph): state machine, ReAct loop, multi-agent orchestrator, tool registry (25 tools), 3-tier memory + autonomous learner, MCP client, personality engine, proactive scheduler, va SSE streaming.

Polyglot voi `apps/api` Fastify/TypeScript (gateway) va `apps/web` React (frontend).

---

## Quick Links

| Document | Description |
|----------|-------------|
| [README.md](../../README.md) | Tong quan du an JARVIS (bilingual VI-EN) |
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
│   ├── orchestrator/        # Multi-agent orchestration: IntentRouter (keyword) chon agent, HandoffManager (agent-to-agent)
│   ├── provider/            # LLM abstraction
│   │   ├── factory/         # Chon provider theo LLM_PROVIDER (gemini/anthropic/deepseek/auto)
│   │   ├── fallback/        # Auto-fallback chain: DeepSeek → Gemini → Claude, cooldown per-provider
│   │   ├── gemini/          # Gemini adapter (genai SDK) + prompt caching
│   │   ├── anthropic/       # Claude adapter (anthropic-sdk-go) + prompt caching
│   │   ├── deepseek/        # DeepSeek adapter (OpenAI-compatible), auto-route flash/pro theo do phuc tap
│   │   └── ollama/          # Local Ollama adapter (dev/offline)
│   ├── tools/                # Tool interface, registry, parallel fan-out — 25 tools (xem bang duoi)
│   ├── memory/               # 3-tier: working (in-msg), episodic (summarize), semantic (extract+store) + autonomous learner
│   ├── personality/          # Personality profile (formality, humor, verbosity) — AdaptPrompt + Learn theo thoi gian
│   ├── proactive/             # Cron scheduler (robfig/cron) — goi prompt dinh ky, tich hop voi engine
│   ├── mcp/                   # MCP client qua subprocess stdin/stdout JSON-RPC 2.0 + auto-discovery tool tu YAML
│   ├── guardrails/            # Circuit breaker (chong stuck loop), tool guard (Read/Write/Destructive + HITL), prompt-injection filter
│   ├── mongo/                  # MongoDB driver (tasks, documents, memories)
│   ├── storage/
│   │   ├── sqlite/            # SQLite (modernc.org/sqlite, pure Go) — conversations/messages/memories offline
│   │   └── chroma/             # In-memory vector store (cosine similarity) — MVP semantic search
│   ├── rag/                    # Voyage AI embedding + Atlas vector search (Parent Document Retrieval, HyDE, LLM rerank)
│   ├── skills/                 # Progressive disclosure engine: ListSkills / LoadSkill / MatchSkill (30 SKILL.md)
│   ├── eval/                   # EvalHarness (exact/contains/regex/LLM-judge) — thu vien, chua wire vao CLI
│   ├── metrics/                # Snapshot metrics: requests, tokens, latency, tool calls, errors (thread-safe)
│   ├── config/                 # Env-based configuration (fail-fast)
│   ├── middleware/              # Context key tenant ID, CORS middleware (dev, khi goi truc tiep khong qua gateway)
│   ├── transport/http/          # SSE chat handler, health/readyz, suggestions endpoint
│   └── observability/           # slog + OpenTelemetry (tracer con noop, chua co exporter that)
├── skills/                     # 30 SKILL.md files (progressive disclosure data)
├── eval/                       # (thu muc rieng cho eval dataset — xem internal/eval de biet harness)
├── go.mod / go.sum
└── Dockerfile                  # Multi-stage distroless (~15MB image)
```

---

## Quick Start

```bash
# Set env (xem env/.env.example o repo root de biet day du bien)
export MONGODB_URI="mongodb+srv://..."
export LLM_PROVIDER="auto"        # gemini | anthropic | deepseek | auto (fallback chain)
export GEMINI_API_KEY="your-key"
export DEEPSEEK_API_KEY="your-key"   # tuy chon — immediate cheap fallback
export ANTHROPIC_API_KEY="your-key"  # tuy chon — last-resort fallback

# HTTP server
go run ./cmd/server

# CLI: one-shot question
go run ./cmd/jarvis ask "Thoi tiet hom nay the nao?"

# CLI: interactive chat (REPL)
go run ./cmd/jarvis chat
```

### Bien env quan trong

| Bien | Mac dinh | Mo ta |
|------|---------|-------|
| `LLM_PROVIDER` | `gemini` | `gemini` \| `anthropic` \| `deepseek` \| `auto` (fallback: DeepSeek → Gemini → Claude) |
| `ENABLE_PLANNING` | `false` | Bat node plan/reflect cho request phuc tap (ton them 1 LLM call truoc token dau) |
| `ENABLE_LEARNER` | `false` | Bat autonomous learner — chay nen trich xuat user facts + knowledge tu hoi thoai |
| `ALLOW_DESTRUCTIVE_TOOLS` | `false` | Cho phep agent tu chay tool destructive (`shell.exec`) khong can xac nhan HITL |
| `OWNER_TENANT_IDS` | (rong) | Danh sach tenant duoc dung nhom tool dac quyen (`file.read/write/search`, `shell.exec`, `git`) — rong = fail-closed, chi tenant `default` (local) dung duoc |
| `MAX_OUTPUT_TOKENS` | `8192` | Tran output token moi lan goi LLM; `0` = khong gioi han |

---

## API

| Method | Path | Description |
|--------|------|--------------|
| `POST` | `/chat` | Send message, receive SSE stream of agent events |
| `GET` | `/healthz` | Liveness check (`{"status":"ok"}`) |
| `GET` | `/readyz` | Readiness check (provider + Mongo ping) |
| `GET` | `/suggestions` | Goi y cau hoi/prompt (dung cho frontend, thuong qua CORS dev-only) |

See [docs/API.md](../../docs/API.md) for full request/response format and all SSE event types.

---

## CLI

```bash
# Start HTTP server
jarvis serve

# One-shot Q&A
jarvis ask "cau hoi cua ban"

# Interactive REPL
jarvis chat
> Xin chao
> /exit    # thoat
```

---

## Tools (25)

| Nhom | Tool | Mo ta ngan |
|------|------|-----------|
| Utility | `calculator`, `datetime`, `timer`, `json`, `translate`, `echo`, `version` | Tinh toan, thoi gian, timer, xu ly JSON, dich, echo test, phien ban |
| File (dac quyen — chi owner tenant) | `file.search`, `file.read`, `file.write` | Doc/ghi/tim file — gioi han duong dan (`AllowedPaths`) |
| Shell/Git (dac quyen) | `shell.exec`, `git` | Chay lenh shell (destructive, can HITL hoac `ALLOW_DESTRUCTIVE_TOOLS`), thao tac git |
| Web | `web.search`, `web.fetch`, `http` | Tim kiem DuckDuckGo, fetch HTML→text, goi HTTP tuy y |
| Memory | `memory.save`, `memory.recall`, `memory.list` | Ghi/truy hoi/liet ke bo nho ngu nghia dai han |
| Notes | `notes.search`, `notes.create` | Ghi chu ca nhan (tenant-scoped) |
| RAG | `rag.search`, `rag.list`, `rag.read` | Tim kiem vector, liet ke DAY DU tai lieu KB, doc noi dung 1 tai lieu |
| Khac | `calendar`, `weather` | Lich, thoi tiet |

Nhom "dac quyen" chi kha dung cho tenant nam trong `OWNER_TENANT_IDS` (xem [`internal/tools/privileged.go`](internal/tools/privileged.go) va [`internal/guardrails`](internal/guardrails)) — vi cac tool nay tac dong len MAY CHAY AGENT, khong scope theo tenant.

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
| **Auto-fallback chain (DeepSeek → Gemini → Claude)** | DeepSeek ~10x re hon Gemini, ~50x re hon Claude — fallback rate-limit voi zero cooldown, khong lam gian doan trai nghiem |
| **FakeProvider** for testing | Deterministic, no network, tests run in ms |
| **errgroup/goroutine** for tool fan-out | `WaitGroup` + error propagation, cleaner API than bare goroutines; timeout mac dinh 60s cho moi tool |
| **json.RawMessage** for schema/args | Deferred parsing, zero-allocation passthrough to LLM |
| **SQLite + in-memory Chroma-style store** | Chay duoc offline/local, khong phu thuoc Mongo Atlas cho dev nhanh; Mongo van la nguon that cho production |
| **MCP qua subprocess JSON-RPC** | Mo rong tool set qua process ngoai (theo chuan Model Context Protocol) ma khong can build lai binary |
| **Owner-tenant gating cho tool dac quyen** | `file.*`/`shell.exec`/`git` tac dong len may chu, khong scope duoc theo tenant → mac dinh fail-closed, chi bat cho tenant khai bao trong `OWNER_TENANT_IDS` |
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
│  (+ plan/reflect neu  │      └──────── tools → model ────────────┘
│   ENABLE_PLANNING)    │
└────┬─────────────┬────┘
     │              │
     ▼              ▼
┌──────────┐   ┌───────────┐   ┌──────────────┐   ┌─────────────┐
│ Provider │   │  Tools     │   │  Guardrails  │   │   Memory     │
│ Gemini   │   │  Registry  │   │  circuit     │   │  3-tier +    │
│ DeepSeek │   │  (25, chay │   │  breaker,    │   │  learner     │
│ Claude   │   │  song song)│   │  tool guard, │   │  (SQLite/    │
│ (auto-   │   │            │   │  HITL,       │   │  Mongo)      │
│ fallback)│   │            │   │  anti-inject │   │              │
└──────────┘   └───────────┘   └──────────────┘   └─────────────┘

Ben canh do: Personality (adapt giong dieu), Proactive (cron prompt dinh ky),
MCP client (tool ngoai qua JSON-RPC), Skills (30 SKILL.md, progressive disclosure).
```
