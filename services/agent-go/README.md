# agent-go — JARVIS Agent Runtime

Agent runtime viet bang **Go** — trai tim cua JARVIS platform. Tu xay tu dau (khong LangGraph): state machine, ReAct loop, tool registry, 3-tier memory, multi-agent orchestrator, SSE streaming.

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
│   ├── server/main.go       # HTTP server entrypoint (SSE chat + health)
│   └── jarvis/main.go       # CLI entrypoint (serve / ask / chat REPL)
├── internal/
│   ├── agent/               # Core engine: State, Nodes, Router, Events, ReAct loop
│   ├── provider/            # LLM abstraction (Gemini, Anthropic, Ollama, Fake)
│   │   ├── factory/         # Provider selection by env
│   │   ├── gemini/          # Gemini adapter (genai SDK)
│   │   ├── anthropic/       # Claude adapter (anthropic-sdk-go)
│   │   └── ollama/          # Local Ollama adapter
│   ├── tools/               # Tool interface, registry, parallel fan-out
│   ├── memory/              # 3-tier: store, recall (search), extract (patterns), summarize
│   ├── orchestrator/        # Multi-agent routing (keyword) + delegation
│   ├── guardrails/          # Tool guard (Read/Write/Destructive) + circuit breaker
│   ├── mongo/               # MongoDB driver (tasks, documents, memories)
│   ├── rag/                 # Voyage AI embedding + Atlas vector search
│   ├── config/              # Env-based configuration (fail-fast)
│   ├── transport/http/      # SSE chat handler + health endpoint
│   └── observability/       # slog + OpenTelemetry (noop phase)
├── skills/                  # SKILL.md files (progressive disclosure data)
├── eval/                    # Agent evaluation harness
├── go.mod / go.sum
└── Dockerfile               # Multi-stage distroless (~15MB image)
```

---

## Quick Start

```bash
# Set env
export MONGODB_URI="mongodb+srv://..."
export LLM_PROVIDER="gemini"
export GEMINI_API_KEY="your-key"

# HTTP server
go run ./cmd/server

# CLI: one-shot question
go run ./cmd/jarvis ask "Thoi tiet hom nay the nao?"

# CLI: interactive chat (REPL)
go run ./cmd/jarvis chat
```

---

## API

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/chat` | Send message, receive SSE stream of agent events |
| `GET` | `/healthz` | Liveness check (`{"status":"ok"}`) |

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

## Testing

```bash
go test ./...                    # all tests
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
| **FakeProvider** for testing | Deterministic, no network, tests run in ms |
| **errgroup** for tool fan-out | `WaitGroup` + error propagation, cleaner API than bare goroutines |
| **json.RawMessage** for schema/args | Deferred parsing, zero-allocation passthrough to LLM |
| **Multi-stage Docker + distroless** | ~15MB image, no shell, minimal attack surface |

---

## Architecture

```
POST /chat (JSON)
      │
      ▼
┌─────────────────────┐
│   Orchestrator      │  Keyword routing: "code", "debug" → code agent
│   (multi-agent)     │  Default: general
└────────┬────────────┘
         │
         ▼
┌─────────────────────┐
│   Engine (ReAct)    │  Loop: recall → summarize → model → route → tools → extract
│                     │         ↑                                    │
│                     │         └────── tools → model ────────────────┘
└────────┬────────────┘
         │
    ┌────┴────┐
    ▼         ▼
┌────────┐ ┌──────────┐
│Provider│ │  Tools   │
│Gemini  │ │Registry  │
│Claude  │ │(parallel)│
└────────┘ └──────────┘
```
