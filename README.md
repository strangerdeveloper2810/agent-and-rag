# J.A.R.V.I.S. — AI Agent Platform

**JARVIS** la mot AI agent runtime tu xay (self-built), viet bang **Go** cho agent engine va **TypeScript (Fastify)** cho API gateway. He thong hoat dong nhu mot tro ly AI ca nhan co kha nang: hoi thoai thong minh, RAG tim kiem tai lieu, quan ly task, ghi nho ngu canh xuyen phien (co autonomous learner), da tac tu (multi-agent), tich hop MCP, va tu dong goi cong cu (tool) de hoan thanh yeu cau.

JARVIS is a self-built AI agent platform featuring a **Go agent runtime** with a custom ReAct loop, an auto-fallback multi-provider LLM layer (Gemini → DeepSeek → Claude), a 25-tool registry with parallel execution, 3-tier memory with an autonomous learner, multi-agent orchestration, MCP client, a personality engine, proactive (cron) scheduling, and SSE streaming. It replaces LangGraph with a hand-crafted state machine for full control, deep learning, and zero framework lock-in.

---

## Architecture

```
 POST /chat (JSON)                          SSE (text/event-stream)
┌─────────────────────┐                 ┌───────────────────────────────┐
│   React SPA (web)   │                 │      Go Agent Runtime          │
│   Vite + Tailwind    │                 │      services/agent-go         │
│   shadcn indigo UI   │                 │                                │
└────────┬─────────────┘                 │  ┌──────────────────────────┐  │
         │                              │  │      Orchestrator         │  │
         ▼                              │  │  (multi-agent, keyword    │  │
┌─────────────────────┐                 │  │   routing + handoff)      │  │
│  Fastify Gateway     │                 │  └────────────┬─────────────┘  │
│  apps/api (TS)       │                 │               │                │
│                      │  HTTP + SSE     │  ┌────────────▼─────────────┐  │
│  - Auth (JWT+OAuth+  │◄───────────────►│  │      Engine (ReAct)       │  │
│    OTP), multi-tenant│   internal      │  │  recall → summarize →     │  │
│  - File upload (S3)  │   localhost     │  │  model → route → tools →  │  │
│  - PDF extraction    │                 │  │  extract → (plan/reflect  │  │
│  - CRUD API          │                 │  │  khi bật ENABLE_PLANNING) │  │
│  - Rate-limit / cache│                 │  └────┬──────────────┬──────┘  │
└────────┬─────────────┘                 │       │              │         │
         │                              │  ┌─────▼──────┐ ┌────▼──────┐  │
         ▼                              │  │Tool Registry│ │ Guardrails │  │
┌──────────────┬──────────────────────┐  │  │25 tools,   │ │ circuit    │  │
│  MongoDB      │ Postgres │ Redis    │  │  │fan-out     │ │ breaker,   │  │
│  Atlas        │ (auth)   │ (cache,  │  │  │song song   │ │ tool guard,│  │
│  conversations│          │ rate-lim)│  │  └────────────┘ │ prompt-    │  │
│  documents    ├──────────┴──────────┤  │                 │ injection  │  │
│  (vectorSearch│  MinIO / S3         │  │  ┌───────────┐  └────────────┘  │
│  tasks, memory│  (uploads)          │  │  │  Memory   │  ┌────────────┐  │
└──────────────┴──────────────────────┘  │  │ 3-tier +  │  │Personality │  │
                                         │  │ learner   │  │  Proactive │  │
                                         │  └───────────┘  │  MCP client│  │
                                         │                 └────────────┘  │
                                         │  Provider Layer (auto-fallback):│
                                         │  Gemini | DeepSeek | Claude |   │
                                         │  Ollama | Fake                 │
                                         └───────────────────────────────┘
```

**Flow**: User message -> Fastify xac thuc + proxy -> Go agent orchestration -> ReAct loop (model <-> tools) -> SSE stream ve -> React render token theo thoi gian thuc.

---

## Quick Start

```bash
# 1. Chuan bi env
cp env/.env.example env/.env              # bien dung chung cho docker + go agent — dien key that
cp apps/web/.env.example apps/web/.env    # VITE_AGENT_URL neu goi thang Go agent tu web
# apps/api doc truc tiep apps/api/.env (chua co .env.example rieng) — copy cac bien
# can thiet tu env/.env.example: MONGODB_URI, PG_CONNECTION_STRING, REDIS_URL,
# ANTHROPIC_API_KEY hoac GOOGLE_API_KEY, VOYAGE_API_KEY, JWT_SECRET*, GOOGLE_CLIENT_*

# 2. Len ha tang dev (MongoDB, Postgres, Redis, MinIO qua Docker)
pnpm docker:dev

# 3. Cai dat & chay full stack (web + api + go agent qua Turborepo)
pnpm install
pnpm dev
```

Mo `http://localhost:3000`. API chay o `:3001`, Go agent o `:3002`.

---

## Project Structure

```
ai-agent-tut/
├── apps/
│   ├── api/                        # Fastify gateway (TypeScript, BFF)
│   │   └── src/
│   │       ├── agent/              # client Go agent (SSE proxy) + langgraph legacy (deprecated)
│   │       ├── modules/            # auth, users, chat, documents, tasks, upload
│   │       ├── database/           # mongo, postgres (auth), redis
│   │       ├── common/             # cache, storage (S3), email, guards, errors
│   │       └── config.ts           # env validation (zod)
│   └── web/                        # React SPA (Vite + Tailwind)
│       └── src/
│           ├── modules/            # chat, documents (feature modules)
│           ├── design-system/      # atoms/molecules/organisms/templates
│           ├── pages/auth/         # login, register, verify-email
│           ├── components/guards/  # AuthGuard, AdminGuard, GuestGuard
│           └── stores/             # zustand (auth store)
├── packages/                       # shared TS workspace packages
│   ├── types/                      # Conversation, Message, ChatEvent, ...
│   ├── http/                       # singleton HTTP client (retry, timeout, interceptors)
│   ├── api-client/                 # typed client wrapping @app/http + @app/types
│   └── ui/                         # shared design-system components (atoms/molecules)
├── services/
│   └── agent-go/                   # JARVIS Go agent runtime
│       ├── cmd/
│       │   ├── server/main.go      # HTTP server entrypoint
│       │   └── jarvis/main.go      # CLI entrypoint (serve / ask / chat)
│       ├── internal/
│       │   ├── agent/              # Engine, State, Nodes (plan/model/route/tools/extract), Router, Events
│       │   ├── provider/           # LLM abstraction
│       │   │   ├── factory/        # chon provider theo env (gemini/anthropic/deepseek/auto)
│       │   │   ├── fallback/       # auto-fallback chain: DeepSeek → Gemini → Claude
│       │   │   ├── gemini/ anthropic/ deepseek/ ollama/  # cac adapter
│       │   ├── tools/              # 25 tools: file, web, rag, memory, notes, shell, git, calendar, ...
│       │   ├── memory/             # 3-tier memory (store, recall, extract, summarize) + learner
│       │   ├── orchestrator/       # multi-agent routing (keyword) + handoff
│       │   ├── personality/        # personality profile (formality, humor, verbosity)
│       │   ├── proactive/          # cron scheduler cho prompt dinh ky
│       │   ├── mcp/                # MCP client (subprocess JSON-RPC) + YAML tool discovery
│       │   ├── guardrails/         # circuit breaker, tool guard, prompt-injection filter, HITL
│       │   ├── mongo/              # MongoDB driver (tasks, documents, memories)
│       │   ├── storage/            # sqlite (conversations local) + chroma (in-memory vector store)
│       │   ├── rag/                # Voyage AI embedding + Atlas vector search (PDR, HyDE, rerank)
│       │   ├── skills/             # progressive disclosure engine (list/load/match SKILL.md)
│       │   ├── eval/               # eval harness (exact/contains/regex/LLM-judge) — thu vien, chua co CLI
│       │   ├── metrics/            # snapshot metrics (requests, tokens, latency, tool calls)
│       │   ├── observability/      # slog + OpenTelemetry (tracer con noop)
│       │   ├── config/             # cau hinh theo env (fail-fast)
│       │   └── transport/http/     # SSE chat handler + health endpoint
│       ├── skills/                 # 30 SKILL.md files (dinh nghia du lieu skill)
│       ├── go.mod
│       └── Dockerfile
├── docs/
│   ├── architecture/                # Architecture deep-dives
│   ├── plans/                       # Design + implementation plans (theo moc thoi gian)
│   └── go-patterns/                 # Go production patterns catalog
├── docker/                          # docker-compose cho dev va deployment
├── deploy/                          # script setup VPS
├── env/                             # .env.example / .env.development dung chung
└── package.json                     # pnpm workspace root (Turborepo)
```

---

## Features Checklist

| Feature | Status | Description |
|---------|:------:|-------------|
| **SSE Streaming** | Done | Token-by-token real-time output; tool call chips in UI |
| **ReAct Agent Loop** | Done | model -> route -> tools -> model -> ... with step limit |
| **Pluggable LLM + Auto-Fallback** | Done | Gemini, Claude, DeepSeek, Ollama; `LLM_PROVIDER=auto` chain: DeepSeek → Gemini → Claude, zero-cooldown failover |
| **Tool System (25 tools)** | Done | Interface-based registry; parallel fan-out via goroutines; per-tool timeout |
| **3-Tier Memory + Learner** | Done | Working (in-msg), episodic (summarize), semantic (extract+store); autonomous learner (`ENABLE_LEARNER`) |
| **Multi-Agent Orchestrator** | Done | Keyword routing; agent-to-agent handoff |
| **MCP Client** | Done | Subprocess JSON-RPC 2.0 + YAML config auto-discovery cho external tool servers |
| **Personality Engine** | Done | Formality/humor/verbosity profile, adapt prompt + tu hoc theo thoi gian |
| **Proactive Scheduler** | Done | Cron-based scheduler goi prompt dinh ky (robfig/cron) |
| **Skills System (30 skills)** | Done | Progressive disclosure qua SKILL.md — list gon trong system prompt, load day du khi trigger |
| **Guardrails** | Done | Tool Kind (Read/Write/Destructive); circuit breaker chong stuck loop; prompt-injection filter; HITL confirmation |
| **RAG Retrieval** | Done | Voyage AI embedding + MongoDB Atlas `$vectorSearch`; Parent Document Retrieval, HyDE, LLM rerank |
| **Prompt Caching** | Done | Gemini + Anthropic provider adapter ho tro cache system/tool defs |
| **Planner Node** | Done | `ENABLE_PLANNING=true` bat node plan/reflect cho request phuc tap (mac dinh tat de tiet kiem 1 LLM call) |
| **Task Management** | Partial | `GET /api/tasks` (chi doc) qua `apps/api/modules/tasks`; **chua co route/tool tao-sua-xoa task, chua co UI task board** |
| **Auth Multi-tenant** | Done | JWT + refresh, Google OAuth, OTP email verify (Resend), tenant isolation |
| **Object Storage** | Done | MinIO/S3 cho file upload (anh, doc, file agent tao) |
| **Eval Harness** | Partial | Package `internal/eval` co day du (exact/contains/regex/LLM-judge) nhung chua wire vao CLI hay dataset that |
| **OpenTelemetry** | Planned | `observability.SetupTracer` hien la no-op provider; metrics in-process (`internal/metrics`) da co, tracing export chua co |

---

## Tech Stack

| Layer | Technology | Purpose |
|-------|-----------|---------|
| **Agent Runtime** | Go 1.25+ | Custom ReAct engine, orchestrator, tool execution, memory, MCP |
| **API Gateway** | Fastify 5 + TypeScript | Auth, file upload, PDF extraction, CRUD, proxy SSE sang Go agent |
| **Frontend** | React 19 + Vite + Tailwind CSS 4 (shadcn, indigo) | Chat UI, auth pages, document management |
| **LLM Providers** | Gemini 3.1 Flash Lite, Claude Haiku 4.5, DeepSeek v4, Ollama (local) | Text generation with tool calling, auto-fallback |
| **Embedding** | Voyage AI voyage-3 (1024d) | Document + memory vectorization |
| **Primary DB** | MongoDB Atlas | Conversations, documents (vector search), tasks, memories |
| **Auth DB** | PostgreSQL 17 | Users, credentials, refresh tokens |
| **Cache / Rate-limit** | Redis 7 | Chat/embedding/tool cache, rate-limit |
| **Object Storage** | MinIO / S3 | Uploaded files (anh, doc, file agent tao) |
| **Local Storage (Go)** | SQLite (modernc.org/sqlite) + in-memory Chroma-style vector store | Conversation/memory offline, semantic search MVP |
| **Streaming** | SSE (Server-Sent Events) | Real-time token + event streaming |
| **Observability** | slog + metrics snapshot + OpenTelemetry (tracer noop) | Structured logging, in-process metrics |
| **Container** | Multi-stage Docker (distroless) | ~15MB production image cho agent-go |
| **Monorepo** | pnpm + Turborepo | Task orchestration, caching, 4 packages + 2 apps + 1 service |

---

## Development Guide

### Prerequisites
- **Go 1.25+** (agent runtime)
- **Node.js 22+** + **pnpm 10+** (gateway + frontend)
- **Docker** (MongoDB/Postgres/Redis/MinIO dev qua `pnpm docker:dev`, hoac tu cai dat rieng)
- API keys: **Gemini** (aistudio.google.com), **Anthropic** (console.anthropic.com) va/hoac **DeepSeek** (platform.deepseek.com), **Voyage AI** (voyageai.com)
- **Google OAuth** client (console.cloud.google.com) + **Resend** API key (OTP email) neu chay auth day du

### Running Individual Services
```bash
# Go agent only (port 3002)
pnpm dev:go

# Fastify gateway only (port 3001)
pnpm dev:api

# React frontend only (port 3000)
pnpm dev:web

# CLI mode: one-shot question
cd services/agent-go && go run ./cmd/jarvis ask "thoi tiet hom nay the nao?"

# CLI mode: interactive chat (REPL)
cd services/agent-go && go run ./cmd/jarvis chat
```

### Running Tests
```bash
pnpm test              # TS: turbo run test (api)
pnpm test:go           # Go: go test ./... -race -count=1 (services/agent-go)
pnpm test:all          # ca hai

pnpm typecheck         # TS type-check
pnpm lint:all          # eslint (TS) + go vet (Go)
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
4. Run `pnpm test:all` and `pnpm typecheck`
5. Commit with conventional commit format
6. Open a PR against `master`

Branch naming: `feat/`, `fix/`, `docs/`, `refactor/`, `test/`.

---

## License

MIT
