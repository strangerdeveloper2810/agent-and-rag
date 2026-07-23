# agent-go

Agent runtime viết bằng **Go** (polyglot với `apps/api` Fastify/TS làm gateway).
Xem thiết kế: [`docs/plans/2026-07-23-go-agent-polyglot-design.md`](../../docs/plans/2026-07-23-go-agent-polyglot-design.md) ·
plan: [`docs/plans/2026-07-23-go-agent-implementation-plan.md`](../../docs/plans/2026-07-23-go-agent-implementation-plan.md).

## Cấu trúc
```
cmd/server/          entrypoint (HTTP server, SSE, graceful shutdown)
internal/
  config/            nạp env
  transport/http/    handlers: /healthz, /chat (SSE), /chat/resume
  agent/             engine: State, nodes, router, loop control
  provider/          Provider interface + gemini/ + anthropic/ (pluggable)
  tools/             Tool interface + registry + từng tool
  rag/               Voyage client + Atlas vector search
  memory/            3 tầng: working / summary / semantic hybrid
  skills/            loader SKILL.md (progressive disclosure)
  guardrails/        input/output/tool guardrails + HITL
  mongo/             client + collections (documents/tasks/memories)
  observability/     OpenTelemetry + slog
skills/              SKILL.md files (data)
eval/                RAG/agent eval harness
```

## Chạy local
```bash
# từ root monorepo
pnpm --filter @app/agent-go dev     # go run ./cmd/server (:3002)
pnpm --filter @app/agent-go test    # go test ./...
# hoặc trực tiếp
cd services/agent-go && go run ./cmd/server
```

Env: `MONGODB_URI` (bắt buộc), `LLM_PROVIDER` (gemini|anthropic), `GEMINI_API_KEY` /
`ANTHROPIC_API_KEY`, `GEMINI_MODEL`, `GOOGLE_THINKING_LEVEL`, `VOYAGE_API_KEY`, `PORT` (3002).
