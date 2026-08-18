# J.A.R.V.I.S. — AI Agent Platform

**JARVIS** là một AI agent runtime tự xây (self-built), viết bằng **Go** cho agent engine và **TypeScript (Fastify)** cho API gateway. Hệ thống hoạt động như một trợ lý AI cá nhân có khả năng: hội thoại thông minh, RAG tìm kiếm tài liệu, quản lý task, ghi nhớ ngữ cảnh xuyên phiên (có autonomous learner), đa tác tử (multi-agent), tích hợp MCP, và tự động gọi công cụ (tool) để hoàn thành yêu cầu.

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

**Flow**: User message -> Fastify xác thực + proxy -> Go agent orchestration -> ReAct loop (model <-> tools) -> SSE stream về -> React render token theo thời gian thực.

---

## Quick Start

```bash
# 1. Chuẩn bị env
cp env/.env.example env/.env              # biến dùng chung cho docker + go agent — điền key thật
cp apps/web/.env.example apps/web/.env    # VITE_AGENT_URL nếu gọi thẳng Go agent từ web
# apps/api đọc trực tiếp apps/api/.env (chưa có .env.example riêng) — copy các biến
# cần thiết từ env/.env.example: MONGODB_URI, PG_CONNECTION_STRING, REDIS_URL,
# ANTHROPIC_API_KEY hoặc GOOGLE_API_KEY, VOYAGE_API_KEY, JWT_SECRET*, GOOGLE_CLIENT_*

# 2. Lên hạ tầng dev (MongoDB, Postgres, Redis, MinIO qua Docker)
pnpm docker:dev

# 3. Cài đặt & chạy full stack (web + api + go agent qua Turborepo)
pnpm install
pnpm dev
```

Mở `http://localhost:3000`. API chạy ở `:3001`, Go agent ở `:3002`.

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
│       │   │   ├── factory/        # chọn provider theo env (gemini/anthropic/deepseek/auto)
│       │   │   ├── fallback/       # auto-fallback chain: DeepSeek → Gemini → Claude
│       │   │   ├── gemini/ anthropic/ deepseek/ ollama/  # các adapter
│       │   ├── tools/              # 25 tools: file, web, rag, memory, notes, shell, git, calendar, ...
│       │   ├── memory/             # 3-tier memory (store, recall, extract, summarize) + learner
│       │   ├── orchestrator/       # multi-agent routing (keyword) + handoff
│       │   ├── personality/        # personality profile (formality, humor, verbosity)
│       │   ├── proactive/          # cron scheduler cho prompt định kỳ
│       │   ├── mcp/                # MCP client (subprocess JSON-RPC) + YAML tool discovery
│       │   ├── guardrails/         # circuit breaker, tool guard, prompt-injection filter, HITL
│       │   ├── mongo/              # MongoDB driver (tasks, documents, memories)
│       │   ├── storage/            # sqlite (conversations local) + chroma (in-memory vector store)
│       │   ├── rag/                # Voyage AI embedding + Atlas vector search (PDR, HyDE, rerank)
│       │   ├── skills/             # progressive disclosure engine (list/load/match SKILL.md)
│       │   ├── eval/               # eval harness (exact/contains/regex/LLM-judge) — thư viện, chưa có CLI
│       │   ├── metrics/            # snapshot metrics (requests, tokens, latency, tool calls)
│       │   ├── observability/      # slog + OpenTelemetry (tracer còn noop)
│       │   ├── config/             # cấu hình theo env (fail-fast)
│       │   └── transport/http/     # SSE chat handler + health endpoint
│       ├── skills/                 # 30 SKILL.md files (định nghĩa dữ liệu skill)
│       ├── go.mod
│       └── Dockerfile
├── docs/
│   ├── architecture/                # Architecture deep-dives
│   ├── plans/                       # Design + implementation plans (theo mốc thời gian)
│   └── go-patterns/                 # Go production patterns catalog
├── docker/                          # docker-compose cho dev và deployment
├── deploy/                          # script setup VPS
├── env/                             # .env.example / .env.development dùng chung
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
| **Personality Engine** | Done | Formality/humor/verbosity profile, adapt prompt + tự học theo thời gian |
| **Proactive Scheduler** | Done | Cron-based scheduler gọi prompt định kỳ (robfig/cron) |
| **Skills System (30 skills)** | Done | Progressive disclosure qua SKILL.md — list gọn trong system prompt, load đầy đủ khi trigger |
| **Guardrails** | Done | Tool Kind (Read/Write/Destructive); circuit breaker chống stuck loop; prompt-injection filter; HITL confirmation |
| **RAG Retrieval** | Done | Voyage AI embedding + MongoDB Atlas `$vectorSearch`; Parent Document Retrieval, HyDE, LLM rerank |
| **Prompt Caching** | Done | Gemini + Anthropic provider adapter hỗ trợ cache system/tool defs |
| **Planner Node** | Done | `ENABLE_PLANNING=true` bật node plan/reflect cho request phức tạp (mặc định tắt để tiết kiệm 1 LLM call) |
| **Task Management** | Partial | `GET /api/tasks` (chỉ đọc) qua `apps/api/modules/tasks`; **chưa có route/tool tạo-sửa-xoá task, chưa có UI task board** |
| **Auth Multi-tenant** | Done | JWT + refresh, Google OAuth, OTP email verify (Resend), tenant isolation |
| **Object Storage** | Done | MinIO/S3 cho file upload (ảnh, doc, file agent tạo) |
| **Eval Harness** | Partial | Package `internal/eval` có đầy đủ (exact/contains/regex/LLM-judge) nhưng chưa wire vào CLI hay dataset thật |
| **OpenTelemetry** | Planned | `observability.SetupTracer` hiện là no-op provider; metrics in-process (`internal/metrics`) đã có, tracing export chưa có |

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
| **Object Storage** | MinIO / S3 | Uploaded files (ảnh, doc, file agent tạo) |
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
- **Docker** (MongoDB/Postgres/Redis/MinIO dev qua `pnpm docker:dev`, hoặc tự cài đặt riêng)
- API keys: **Gemini** (aistudio.google.com), **Anthropic** (console.anthropic.com) và/hoặc **DeepSeek** (platform.deepseek.com), **Voyage AI** (voyageai.com)
- **Google OAuth** client (console.cloud.google.com) + **Resend** API key (OTP email) nếu chạy auth đầy đủ

### Running Individual Services
```bash
# Go agent only (port 3002)
pnpm dev:go

# Fastify gateway only (port 3001)
pnpm dev:api

# React frontend only (port 3000)
pnpm dev:web

# CLI mode: one-shot question
cd services/agent-go && go run ./cmd/jarvis ask "thời tiết hôm nay thế nào?"

# CLI mode: interactive chat (REPL)
cd services/agent-go && go run ./cmd/jarvis chat
```

### Running Tests
```bash
pnpm test              # TS: turbo run test (api)
pnpm test:go           # Go: go test ./... -race -count=1 (services/agent-go)
pnpm test:all          # cả hai

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
