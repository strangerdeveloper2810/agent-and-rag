# Bảng mapping nhanh: khái niệm → file

← [Về mục lục](./README.md)

| Khái niệm | File |
|---|---|
| Entrypoint BFF | `apps/api/src/server.ts`, `app.ts`, `config.ts` |
| Entrypoint agent-go (production) | `services/agent-go/cmd/server/main.go` |
| Entrypoint agent-go (CLI: serve/ask/chat/eval/cost) | `services/agent-go/cmd/jarvis/main.go` |
| Auth + tenant identity | `apps/api/src/common/guards/auth.guard.ts` |
| Proxy BFF → agent-go | `apps/api/src/agent/client/go-agent.client.ts` |
| Tenant middleware (agent-go) | `services/agent-go/internal/middleware/tenant.go` |
| Engine ReAct loop + checkpoint | `services/agent-go/internal/agent/engine.go`, `node_*.go`, `resume.go`, `state.go` |
| Orchestrator + sticky agent | `services/agent-go/internal/orchestrator/orchestrator.go` |
| Provider fallback chain | `services/agent-go/internal/provider/factory/factory.go`, `fallback/fallback.go` |
| RouterProvider (local/cloud) | `services/agent-go/internal/provider/router/router.go` |
| Pricing / cost ledger | `services/agent-go/internal/provider/pricing/`, `internal/storage/sqlite/cost_ledger.go` |
| Provider adapter (Ollama/openai_compat) | `services/agent-go/internal/provider/ollama/`, `internal/provider/openai_compat/` |
| Memory + Learner | `services/agent-go/internal/memory/{store,recall,learner,reflection}.go` |
| MCP client (2 chế độ) | `services/agent-go/internal/mcp/{sse,discovery}.go` |
| MCP server (mới) | `services/agent-go/internal/mcp/server.go`, `registry.go` |
| MCP config phía BFF | `apps/api/src/modules/users/dto/mcp-server.dto.ts`, Postgres `user_mcp_servers` |
| Guardrails | `services/agent-go/internal/guardrails/` |
| Shell allowlist + sandbox Docker | `services/agent-go/internal/tools/shell.go`, `shell_sandbox_docker.go` |
| Checkpoint/resume storage | `services/agent-go/internal/storage/sqlite/paused_runs.go` |
| Resume HTTP handler | `services/agent-go/internal/transport/http/chat_resume.go` |
| Telegram channel | `services/agent-go/internal/transport/telegram/telegram.go` |
| Observability (OTel thật) | `services/agent-go/internal/observability/observability.go` |
| Security model (privileged tools, sandbox, MCP auth) | `services/agent-go/docs/security-model.md` |
| RAG ingest (BFF) | `apps/api/src/modules/documents/` |
| RAG tra cứu (agent-go) | `services/agent-go/internal/tools/rag*.go`, `internal/rag/` |
| (Legacy) LangGraph/LangChain | `apps/api/src/agent/{graph,lc-tools,graph-runner}.ts` — xem [`docs/architecture-backend-agent.md`](../../architecture-backend-agent.md) |
