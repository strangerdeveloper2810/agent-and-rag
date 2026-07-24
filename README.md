# J.A.R.V.I.S. — AI Agent Platform

**JARVIS** la mot AI agent runtime tu xay (self-built), viet bang **Go** cho agent engine va **TypeScript (Fastify)** cho API gateway. He thong hoat dong nhu mot tro ly AI ca nhan co kha nang: hoi thoai thong minh, RAG tim kiem tai lieu, quan ly task, ghi nho ngu canh xuyen phien, va tu dong goi cong cu (tool) de hoan thanh yeu cau.

JARVIS is a self-built AI agent platform featuring a **Go agent runtime** with a custom ReAct loop, pluggable LLM providers (Gemini/Claude), a tool registry with parallel execution, 3-tier memory, multi-agent orchestration, and SSE streaming. It replaces LangGraph with a hand-crafted state machine for full control, deep learning, and zero framework lock-in.

---

## Architecture

```
 POST /chat (JSON)                          SSE (text/event-stream)
┌─────────────────────┐                 ┌──────────────────────────┐
│   React SPA (web)   │                 │    Go Agent Runtime       │
│   Vite + Tailwind   │                 │    services/agent-go      │
└────────┬────────────┘                 │                          │
         │                              │  ┌────────────────────┐  │
         ▼                              │  │   Orchestrator     │  │
┌─────────────────────┐                 │  │  (multi-agent)     │  │
│  Fastify Gateway     │                 │  │  intent routing    │  │
│  apps/api (TS)       │                 │  └────────┬───────────┘  │
│                      │  HTTP + SSE     │           │              │
│  - File upload       │◄───────────────►│  ┌────────▼───────────┐  │
│  - PDF extraction    │   internal      │  │   Engine (ReAct)    │  │
│  - CRUD API          │   localhost     │  │                     │  │
│  - Auth / Rate-limit │                 │  │  recall → summarize │  │
└────────┬────────────┘                 │  │    → model → route  │  │
         │                              │  │    → tools → extract│  │
         ▼                              │  └────────┬───────────┘  │
┌─────────────────────────────────────┐  │           │              │
│         MongoDB Atlas               │  │  ┌────────▼───────────┐  │
│  - conversations, messages          │  │  │   Tool Registry    │  │
│  - documents (vector search)        │  │  │   fan-out parallel │  │
│  - tasks, memories                  │  │  └────────────────────┘  │
└─────────────────────────────────────┘  │                          │
                                         │  Provider Layer:         │
                                         │  Gemini | Claude | Fake  │
                                         └──────────────────────────┘
```

**Flow**: User message -> Fastify validates & proxies -> Go agent orchestration -> ReAct loop (model <-> tools) -> SSE stream back -> React renders tokens in real-time.

---

## Quick Start

```bash
# 1. Set environment variables
cp apps/api/.env.example apps/api/.env
# Edit .env: MONGODB_URI, GEMINI_API_KEY (or ANTHROPIC_API_KEY), VOYAGE_API_KEY

# 2. Install & run (full stack)
pnpm install
pnpm dev
```

Then open `http://localhost:3000` in your browser. The API runs on `:3001`, Go agent on `:3002`.

---

## Project Structure

```
ai-agent-tut/
├── apps/
│   ├── api/                    # Fastify gateway (TypeScript)
│   │   └── src/
│   │       ├── agent/          # LangGraph (legacy) + graph-runner
│   │       ├── modules/        # conversations, documents, tasks, chat
│   │       ├── middleware/     # error handler
│   │       └── lib/            # mongo, claude, voyage, errors
│   └── web/                    # React SPA (Vite + Tailwind)
│       └── src/
│           ├── components/     # chat, documents, tasks UI
│           └── lib/            # http client, SSE parser
├── services/
│   └── agent-go/               # JARVIS Go agent runtime
│       ├── cmd/
│       │   ├── server/main.go  # HTTP server entrypoint
│       │   └── jarvis/main.go  # CLI entrypoint (serve/ask/chat)
│       ├── internal/
│       │   ├── agent/          # Engine, State, Nodes, Router, Events
│       │   ├── provider/       # LLM abstraction (Gemini, Anthropic, Fake)
│       │   │   ├── factory/    # Provider selection by env
│       │   │   ├── gemini/     # Gemini adapter (genai SDK)
│       │   │   └── anthropic/  # Claude adapter
│       │   ├── tools/          # Tool interface, registry, built-in tools
│       │   ├── memory/         # 3-tier memory (store, recall, extract, summarize)
│       │   ├── orchestrator/   # Multi-agent routing & delegation
│       │   ├── guardrails/     # Tool guard + circuit breaker
│       │   ├── mongo/          # MongoDB driver (tasks, documents, memories)
│       │   ├── rag/            # Voyage AI embedding + vector search
│       │   ├── config/         # Environment configuration
│       │   ├── transport/http/ # SSE chat handler + health
│       │   └── observability/  # slog + OpenTelemetry
│       ├── skills/             # SKILL.md files (progressive disclosure)
│       ├── go.mod
│       └── Dockerfile
├── docs/
│   ├── architecture/           # Architecture deep-dives
│   ├── plans/                  # Design + implementation plans
│   └── go-patterns/            # Go production patterns catalog
└── package.json                # pnpm workspace root
```

---

## Features Checklist

| Feature | Status | Description |
|---------|:------:|-------------|
| **SSE Streaming** | Done | Token-by-token real-time output; tool call chips in UI |
| **ReAct Agent Loop** | Done | model -> route -> tools -> model -> ... with step limit |
| **Pluggable LLM** | Done | Gemini 3.1 Flash Lite (default), Claude Haiku; env-switchable |
| **Tool System** | Done | Interface-based registry; parallel fan-out via errgroup |
| **3-Tier Memory** | Done | Working (in-msg), episodic (summarize), semantic (extract+store) |
| **Multi-Agent Orchestrator** | Done | Keyword routing; agent-to-agent delegation |
| **File Tools** | Done | file.search (glob), file.read (path-restricted) |
| **Web Tools** | Done | web.search (DuckDuckGo), web.fetch (HTML->text) |
| **RAG Retrieval** | Done | Voyage AI embedding + MongoDB Atlas $vectorSearch |
| **Task Management** | Done | CRUD via tool calls (createTask, listTasks, updateTask, deleteTask) |
| **Guardrails** | Done | Tool Kind (Read/Write/Destructive); circuit breaker; HITL interrupt |
| **Prompt Caching** | Planned (P6) | System + tool defs cached for cost reduction |
| **Planner Node** | Planned (P8) | Task decomposition for complex requests |
| **Skills System** | Planned (P9) | Progressive disclosure via SKILL.md files |
| **OpenTelemetry** | Planned (P11) | Full tracing + metrics export |
| **Eval Harness** | Planned (P13) | Automated correctness + performance evaluation |

---

## Tech Stack

| Layer | Technology | Purpose |
|-------|-----------|---------|
| **Agent Runtime** | Go 1.25+ | Custom ReAct engine, tool execution, memory |
| **API Gateway** | Fastify 5 + TypeScript | File upload, PDF extraction, CRUD, auth, rate-limit |
| **Frontend** | React 19 + Vite + Tailwind CSS 4 | Chat UI, document management, task board |
| **LLM Providers** | Gemini 3.1 Flash Lite, Claude Haiku | Text generation with tool calling |
| **Embedding** | Voyage AI voyage-3 (1024d) | Document + memory vectorization |
| **Database** | MongoDB Atlas | Conversations, documents (vector search), tasks, memories |
| **Streaming** | SSE (Server-Sent Events) | Real-time token + event streaming |
| **Observability** | slog + OpenTelemetry (planned) | Structured logging, tracing |
| **Container** | Multi-stage Docker (distroless) | ~15MB production image |
| **Monorepo** | pnpm + Turborepo | Task orchestration, caching |

---

## Development Guide

### Prerequisites
- **Go 1.25+** (for agent runtime)
- **Node.js 22+** + **pnpm 9+** (for gateway + frontend)
- **MongoDB Atlas** cluster (free tier works)
- API keys: **Gemini** (https://aistudio.google.com) or **Anthropic** (https://console.anthropic.com), and **Voyage AI** (https://voyageai.com)

### Running Individual Services
```bash
# Go agent only (port 3002)
cd services/agent-go
go run ./cmd/server

# Fastify gateway only (port 3001)
pnpm --filter @app/api dev

# React frontend only (port 3000)
pnpm --filter @app/web dev

# CLI mode: one-shot question
cd services/agent-go && go run ./cmd/jarvis ask "thoi tiet hom nay the nao?"

# CLI mode: interactive chat (REPL)
cd services/agent-go && go run ./cmd/jarvis chat
```

### Running Tests
```bash
# Go tests
cd services/agent-go && go test ./...

# TypeScript tests
pnpm test

# Type checking
pnpm typecheck
```

### Code Style
- **Go**: `gofmt` + `go vet` + table-driven tests
- **TypeScript**: ESLint + Prettier + Zod for runtime validation
- **Commits**: Conventional commits (`feat:`, `fix:`, `docs:`, `refactor:`, `test:`)

---

## Contributing

1. Fork the repo
2. Create a feature branch: `git checkout -b feat/your-feature`
3. Write code + tests (table-driven for Go, Vitest for TS)
4. Run `go test ./...` and `pnpm typecheck`
5. Commit with conventional commit format
6. Open a PR against `master`

Branch naming: `feat/`, `fix/`, `docs/`, `refactor/`, `test/`.

---

## License

MIT
