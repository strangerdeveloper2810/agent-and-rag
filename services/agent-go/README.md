# agent-go — JARVIS Agent Runtime

Agent runtime viết bằng **Go** — trái tim của JARVIS platform. Tự xây từ đầu (không LangGraph): state machine, ReAct loop, multi-agent orchestrator, tool registry (25 tools), 3-tier memory + autonomous learner (bền vững qua restart), MCP client **và server**, personality engine, proactive scheduler, chống phình context (dedup + ngân sách + nén bằng LLM thật), checkpoint/resume (crash-safe, mọi node), cost ledger per-tenant, OpenTelemetry tracing thật, kênh Telegram, và SSE streaming.

Polyglot với `apps/api` Fastify/TypeScript (gateway) và `apps/web` React (frontend). Xem lý do không dùng LangChain/LangGraph ở [README gốc](../../README.md#why-not-langchain--langgraph).

---

## Quick Links

| Document | Description |
|----------|-------------|
| [README.md](../../README.md) | Tổng quan dự án JARVIS (bilingual VI-EN) |
| [docs/security-model.md](docs/security-model.md) | Mô hình tin cậy: tool đặc quyền, sandbox Docker (trade-off docker.sock), MCP server auth |
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
│   ├── server/main.go       # HTTP server entrypoint (SSE chat + health + suggestions + resume + MCP server + Telegram)
│   └── jarvis/main.go       # CLI entrypoint (serve / ask / chat REPL / eval / cost)
├── internal/
│   ├── agent/               # Core engine: State, Nodes (plan/model/route/tools/extract), Router, Events, ReAct loop,
│   │                         # context compaction, checkpoint/resume (mọi node, không chỉ HITL), cost ledger hook
│   ├── orchestrator/        # Multi-agent orchestration: IntentRouter (keyword) chọn agent, HandoffManager (agent-to-agent)
│   ├── provider/            # LLM abstraction
│   │   ├── factory/         # Chọn provider theo LLM_PROVIDER (gemini/anthropic/deepseek/ollama/openai_compat/router/auto)
│   │   ├── fallback/        # Auto-fallback chain: DeepSeek → Gemini → Claude, cooldown per-provider
│   │   ├── router/          # RouterProvider: local (Ollama/openai_compat) khi ThinkingOff+không tool, cloud phần còn lại
│   │   ├── pricing/         # Bảng giá ước tính USD/1M token cho cost ledger (override qua PRICING_OVERRIDE_JSON)
│   │   ├── gemini/          # Gemini adapter (genai SDK) + prompt caching
│   │   ├── anthropic/       # Claude adapter (anthropic-sdk-go) + prompt caching
│   │   ├── deepseek/        # DeepSeek adapter (OpenAI-compatible), auto-route flash/pro theo độ phức tạp
│   │   ├── ollama/          # Local Ollama adapter (role "tool" chuẩn, không giả role "user")
│   │   └── openai_compat/   # Adapter cho server local kiểu OpenAI-compatible (vLLM, llama.cpp, LM Studio...)
│   ├── tools/                # Tool interface, registry, parallel fan-out — 25 tools (xem bảng dưới)
│   │                         # shell.exec: allowlist thật + kill cả process group + sandbox Docker opt-in
│   ├── memory/               # 3-tier: working (in-msg), episodic (summarize), semantic (extract+store) + autonomous learner + Mongo persistence (sống sót qua restart)
│   ├── personality/          # Personality profile (formality, humor, verbosity) — AdaptPrompt + Learn theo thời gian
│   ├── proactive/             # Cron scheduler (robfig/cron) — gọi prompt định kỳ, tích hợp với engine
│   ├── mcp/                   # MCP CLIENT (subprocess/SSE JSON-RPC 2.0 + auto-discovery) VÀ MCP SERVER
│   │                         # (POST /mcp, expose whitelist tool an toàn, hard-exclude tool đặc quyền, auth loopback/API key)
│   ├── transport/
│   │   ├── http/               # SSE chat handler, health/readyz, suggestions, /chat/resume
│   │   └── telegram/           # Kênh Telegram long-polling (tenant tự cô lập theo telegram:<chatID>)
│   ├── guardrails/            # Circuit breaker (chống stuck loop), tool guard (Read/Write/Destructive + HITL), prompt-injection filter
│   ├── mongo/                  # MongoDB driver (tasks, documents, memories)
│   ├── storage/
│   │   ├── sqlite/            # SQLite (modernc.org/sqlite, pure Go) — conversations/messages/memories,
│   │   │                       # paused_runs (checkpoint/resume), cost_ledger (chi phí per-tenant)
│   │   └── chroma/             # In-memory vector store (cosine similarity) — MVP semantic search
│   ├── rag/                    # Voyage AI embedding + Atlas vector search (Parent Document Retrieval, HyDE, LLM rerank)
│   ├── skills/                 # Progressive disclosure engine: ListSkills / LoadSkill / MatchSkill (30 SKILL.md)
│   ├── eval/                   # EvalHarness (exact/contains/regex/LLM-judge) — wired vào CLI qua `jarvis eval`
│   ├── metrics/                # Snapshot metrics: requests, tokens, latency, tool calls, errors (thread-safe)
│   ├── config/                 # Env-based configuration (fail-fast)
│   ├── middleware/              # Context key tenant ID, CORS middleware (dev, khi gọi trực tiếp không qua gateway)
│   └── observability/           # slog + OpenTelemetry THẬT (stdouttrace mặc định, OTLP HTTP nếu có OTEL_EXPORTER_OTLP_ENDPOINT)
├── docs/security-model.md      # Mô hình tin cậy: privileged tools, sandbox Docker, MCP server auth
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
export LLM_PROVIDER="auto"        # gemini | anthropic | deepseek | ollama | openai_compat | router | auto
export GEMINI_API_KEY="your-key"
export DEEPSEEK_API_KEY="your-key"   # tuỳ chọn — immediate cheap fallback
export ANTHROPIC_API_KEY="your-key"  # tuỳ chọn — last-resort fallback

# HTTP server (production entrypoint — Telegram/MCP server/resume wired ở đây)
go run ./cmd/server

# CLI: one-shot question
go run ./cmd/jarvis ask "Thời tiết hôm nay thế nào?"

# CLI: interactive chat (REPL)
go run ./cmd/jarvis chat

# CLI: chạy bộ eval case (JSON) qua agent thật
go run ./cmd/jarvis eval path/to/cases.json

# CLI: xem chi phí ước tính + tiết kiệm của 1 tenant (cần ENABLE_COST_LEDGER=true khi chạy server)
go run ./cmd/jarvis cost <tenantID>
```

### Biến env quan trọng

| Biến | Mặc định | Mô tả |
|------|---------|-------|
| `LLM_PROVIDER` | `gemini` | `gemini` \| `anthropic` \| `deepseek` \| `ollama` \| `openai_compat` \| `router` \| `auto` (fallback: DeepSeek → Gemini → Claude) |
| `ROUTER_LOCAL_BACKEND` | `ollama` | Khi `LLM_PROVIDER=router`: backend local dùng cho request `ThinkingOff` + không tool (`ollama` \| `openai_compat`), phần còn lại route sang chuỗi cloud (`auto`) |
| `SHELL_ALLOWED_COMMANDS` | `git,ls,grep,cat,find,pwd,echo,wc,head,tail,diff` | Allowlist lệnh cho `shell.exec` — KHÔNG thêm interpreter (`node`/`python`/`bash`) trừ khi hiểu rõ rủi ro RCE |
| `SHELL_SANDBOX` | (rỗng = tắt) | `docker` bật chạy `shell.exec` qua container ephemeral giới hạn resource/network — đọc [docs/security-model.md](docs/security-model.md) trước khi bật (cần mount `docker.sock`, không phải cô lập tuyệt đối) |
| `TELEGRAM_BOT_TOKEN` | (rỗng) | Bật kênh Telegram long-polling khi khác rỗng — để trống thì chỉ chạy kênh HTTP/SSE như trước |
| `MCP_API_KEY` | (rỗng) | Auth cho `POST /mcp` (JARVIS làm MCP server) — rỗng thì chỉ chấp nhận request loopback, set key để gọi từ máy khác qua `Authorization: Bearer <key>` |
| `ENABLE_COST_LEDGER` | `false` | Ghi bảng `cost_ledger` (chi phí ước tính per-tenant) — TẮT mặc định vì là side-effect ghi SQLite thêm cho mọi request |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | (rỗng) | Có giá trị → xuất trace qua OTLP HTTP; rỗng → xuất ra stdout (`stdouttrace`) |
| `ENABLE_PLANNING` | `false` | Bật node plan/reflect cho request phức tạp (tốn thêm 1 LLM call trước token đầu) |
| `ENABLE_LEARNER` | `false` | Bật autonomous learner — chạy nền trích xuất user facts + knowledge từ hội thoại |
| `ALLOW_DESTRUCTIVE_TOOLS` | `false` | Cho phép agent tự chạy tool destructive (`shell.exec`) không cần xác nhận HITL |
| `OWNER_TENANT_IDS` | (rỗng) | Danh sách tenant được dùng nhóm tool đặc quyền (`file.read/write/search`, `shell.exec`, `git`) — rỗng = fail-closed, chỉ tenant `default` (local) dùng được |
| `MAX_OUTPUT_TOKENS` | `8192` | Trần output token mỗi lần gọi LLM; `0` = không giới hạn |
| `MAX_CONTEXT_TOKENS` | `100000` | Ngân sách token context trước khi nén (dedup xong vẫn vượt) kích hoạt; cũng là ngân sách FE dùng để tính % và gợi ý mở hội thoại mới; `0` = không giới hạn |
| `MAX_TOTAL_TOOL_OUTPUT` | `60000` | Ngân sách ký tự tool-output cộng dồn qua cả lượt chạy (nhiều tool call); `0` = không giới hạn |

---

## API

| Method | Path | Description |
|--------|------|--------------|
| `POST` | `/chat` | Send message, receive SSE stream of agent events |
| `POST` | `/chat/resume` | Resume 1 run đã dừng giữa chừng (interrupt HITL hoặc crash) theo `run_id` — `answer` optional, chỉ bắt buộc khi run đang chờ trả lời interrupt |
| `POST` | `/mcp` | JARVIS làm **MCP server** (JSON-RPC 2.0, Streamable HTTP) — expose whitelist tool an toàn cho MCP client khác gọi vào. Auth: loopback mặc định, hoặc `MCP_API_KEY` |
| `GET` | `/healthz` | Liveness check (`{"status":"ok"}`) |
| `GET` | `/readyz` | Readiness check (provider + Mongo ping) |
| `GET` | `/suggestions` | Gợi ý câu hỏi/prompt (dùng cho frontend, thường qua CORS dev-only) |

See [docs/API.md](../../docs/API.md) for full request/response format and all SSE event types (including `contextTokens`/`contextBudget` on the `done` event, used by the frontend to suggest starting a new chat).

---

## CLI

```bash
# Start HTTP server (production entrypoint)
jarvis serve

# One-shot Q&A
jarvis ask "câu hỏi của bạn"

# Interactive REPL
jarvis chat
> Xin chào
> /exit    # thoát

# Chạy bộ eval case (JSON array of EvalCase) qua agent thật, in báo cáo pass/fail
jarvis eval path/to/cases.json

# Xem tổng chi phí ước tính + tiết kiệm của 1 tenant (đọc bảng cost_ledger)
jarvis cost <tenantID>
```

---

## Tools (25)

| Nhóm | Tool | Mô tả ngắn |
|------|------|-----------|
| Utility | `calculator`, `datetime`, `timer`, `json`, `translate`, `echo`, `version`, `ask_user` | Tính toán, thời gian, timer, xử lý JSON, dịch, echo test, phiên bản, hỏi lại user (HITL) |
| File (đặc quyền — chỉ owner tenant) | `file.search`, `file.read`, `file.write` | Đọc/ghi/tìm file — giới hạn đường dẫn (`AllowedPaths`) + nest theo tenant |
| Shell/Git (đặc quyền) | `shell.exec`, `git` | Chạy lệnh shell (allowlist + kill process group + sandbox Docker opt-in), thao tác git |
| Web | `web.search`, `web.fetch`, `http` | Tìm kiếm DuckDuckGo, fetch HTML→text, gọi HTTP tuỳ ý |
| Memory | `memory.save`, `memory.recall`, `memory.list` | Ghi/truy hồi/liệt kê bộ nhớ ngữ nghĩa dài hạn — dùng CHUNG 1 store với pipeline recall/extract tự động (không còn 2 kho tách biệt) |
| Notes | `notes.search`, `notes.create` | Ghi chú cá nhân (tenant-scoped) |
| RAG | `rag.search`, `rag.list`, `rag.read` | Tìm kiếm vector, liệt kê ĐẦY ĐỦ tài liệu KB, đọc nội dung 1 tài liệu |
| Khác | `calendar`, `weather` | Lịch, thời tiết |

Nhóm "đặc quyền" chỉ khả dụng cho tenant nằm trong `OWNER_TENANT_IDS` (xem [`internal/tools/privileged.go`](internal/tools/privileged.go) và [`internal/guardrails`](internal/guardrails)) — vì các tool này tác động lên MÁY CHẠY AGENT, không scope theo tenant. Nhóm này bị **hard-exclude tuyệt đối** khỏi MCP server (`internal/mcp/server.go`) — đường vào đó không có khái niệm owner-tenant nên phải nghiêm hơn kênh chat.

---

## Testing

```bash
go test ./...                    # all tests (113+ file _test.go)
go test -v ./internal/agent/...  # engine tests with FakeProvider, checkpoint/resume
go test -v ./internal/tools/...  # tool execution + concurrency + sandbox
go test -v ./internal/mcp/...    # MCP client + server (hard-exclude privileged tools, auth)
go test -race ./...              # race detection
go test -cover ./...             # coverage
```

---

## Design Decisions

| Decision | Why |
|----------|-----|
| **Self-built engine** (no LangGraph) | Full control, learning, zero dependency on Python/TS framework — see [Why Not LangChain?](../../README.md#why-not-langchain--langgraph) |
| **SSE, not WebSocket** | Unidirectional server->client, browser-native EventSource, simpler |
| **Stateless agent** | History sent per request — no sticky sessions, horizontal scaling |
| **Pluggable providers** | `Provider` interface — swap LLM via env var, zero engine changes |
| **Auto-fallback chain (DeepSeek → Gemini → Claude)** | DeepSeek ~10x rẻ hơn Gemini, ~50x rẻ hơn Claude — fallback rate-limit với zero cooldown, không làm gián đoạn trải nghiệm |
| **RouterProvider (local + cloud)** | `ThinkingOff` + không tool → local (Ollama/openai_compat), miễn phí; còn lại → cloud chain. Fallback local→cloud CHỈ khi lỗi ngay từ đầu, KHÔNG khi lỗi giữa stream (tránh trộn response 2 nguồn) |
| **Checkpoint mọi node, không chỉ HITL interrupt** | Ban đầu chỉ resume được khi cố ý hỏi user; tổng quát hoá để crash/restart giữa chừng cũng resume được — checkpoint đồng bộ, fail-safe (lỗi ghi chỉ log warn, không chặn response). Giới hạn còn lại: nhiều tool chạy song song trong CÙNG 1 batch chưa có idempotency key riêng |
| **Cost ledger minh bạch, không hardcode giá là sự thật** | Giá LLM đổi liên tục — bảng giá ghi rõ là ước tính, override được qua `PRICING_OVERRIDE_JSON`, không coi số $ tính ra là chính xác tuyệt đối |
| **MCP server hard-exclude tool đặc quyền, không có ngoại lệ** | Đây là đường vào MỚI không đi qua `node_tools.go`/owner-tenant gate — phải nghiêm hơn kênh chat thường, không tin caller filter tuyệt đối (defense-in-depth) |
| **Sandbox Docker: cải thiện, không phải cô lập tuyệt đối** | Mount `docker.sock` để spawn sibling container về bản chất mở rộng quyền tương đương root host — ghi rõ trade-off trong docs thay vì giả vờ đây là an toàn tuyệt đối; TẮT mặc định |
| **FakeProvider** for testing | Deterministic, no network, tests run in ms |
| **errgroup/goroutine** for tool fan-out | `WaitGroup` + error propagation, cleaner API than bare goroutines; timeout mặc định 60s cho mỗi tool |
| **json.RawMessage** for schema/args | Deferred parsing, zero-allocation passthrough to LLM |
| **Real LLM compaction, not a placeholder** | Khi context vượt `MAX_CONTEXT_TOKENS`, tin nhắn cũ được TÓM TẮT THẬT bằng model rẻ/nhanh (fallback trung thực khi lỗi/timeout — không giả vờ đã tóm tắt) thay vì chèn text cứng "đã tóm tắt" mà không tóm tắt gì |
| **Go quyết định số liệu, FE quyết định ngưỡng** | Event `done` chỉ báo `contextTokens`/`contextBudget` thô; ngưỡng cảnh báo (80%) và copy UI nằm ở frontend — đổi ngưỡng không cần redeploy Go |
| **SQLite + in-memory Chroma-style store** | Chạy được offline/local, không phụ thuộc Mongo Atlas cho dev nhanh; Mongo vẫn là nguồn thật cho production |
| **MCP qua subprocess/SSE JSON-RPC (client) và Streamable HTTP (server)** | Mở rộng tool set qua process/server ngoài (client), đồng thời expose tool của chính JARVIS cho client khác (server) — cùng chuẩn Model Context Protocol |
| **Owner-tenant gating cho tool đặc quyền** | `file.*`/`shell.exec`/`git` tác động lên máy chủ, không scope được theo tenant → mặc định fail-closed, chỉ bật cho tenant khai báo trong `OWNER_TENANT_IDS` |
| **Multi-stage Docker + distroless** | ~15MB image, no shell, minimal attack surface |

---

## Architecture

```
POST /chat (JSON)          Telegram (long-polling)         MCP client khác
      │                           │                          (POST /mcp)
      ▼                           ▼                              │
┌─────────────────────────────────────────┐                      │
│   Orchestrator (multi-agent)              │◄─────────────────────
│   IntentRouter (keyword) · HandoffManager │
└────────────────────┬──────────────────────┘
                      │
                      ▼
┌──────────────────────────────────────────┐   checkpoint mỗi node
│   Engine (ReAct)                          │──────────────┐
│   recall → summarize → model → route →    │              ▼
│   tools → extract (+ plan/reflect nếu     │      ┌───────────────┐
│   ENABLE_PLANNING)                        │      │ paused_runs    │
└───┬──────────┬──────────┬─────────┬───────┘      │ (SQLite —      │
    │          │          │         │              │ /chat/resume)  │
    ▼          ▼          ▼         ▼              └───────────────┘
┌────────┐ ┌────────┐ ┌──────────┐ ┌─────────┐
│Provider│ │ Tools  │ │Guardrails│ │ Memory   │      cost ledger (opt-in)
│Gemini/ │ │Registry│ │circuit   │ │3-tier +  │──────► cost_ledger (SQLite)
│Claude/ │ │(25,    │ │breaker,  │ │learner   │        jarvis cost <tenant>
│DeepSeek│ │song    │ │tool guard│ │(SQLite/  │
│/Ollama/│ │song,   │ │HITL,     │ │Mongo,    │
│openai_ │ │sandbox │ │anti-     │ │persist)  │
│compat/ │ │Docker  │ │inject)   │ │          │
│Router  │ │opt-in) │ │          │ │          │
└────────┘ └────────┘ └──────────┘ └─────────┘

Bên cạnh đó: Personality (adapt giọng điệu), Proactive (cron prompt định kỳ),
Skills (30 SKILL.md, progressive disclosure), OpenTelemetry (stdouttrace/OTLP).
```
