# agent-go — Sơ đồ package, vòng ReAct, Orchestrator

← [Về mục lục](./README.md)

## Sơ đồ package

```
cmd/
  server/main.go     — HTTP entrypoint (production): wiring provider, registry, orchestrator, learner,
                        checkpoint/resume store, cost ledger, Telegram bot, MCP server, routes
  jarvis/main.go      — CLI: serve / ask / chat (REPL) / eval / cost — chạy agent không cần HTTP server

internal/
  agent/          — Engine (ReAct state machine), State, Node (model/tools/recall/summarize/extract/reflect),
                    Router, checkpoint/resume (mọi node), cost ledger hook — xem agent-go-resilience.md,
                    agent-go-providers.md
  orchestrator/    — multi-agent: chọn agent (general/code/research) theo keyword + STICKY AGENT + handoff
  provider/        — LLM abstraction; factory (chọn theo env) + fallback (auto-chain) + router (local/cloud)
                    + pricing (bảng giá) + adapter (gemini/anthropic/deepseek/ollama/openai_compat)
                    — xem agent-go-providers.md
  tools/           — 25 tool: file, web, rag.*, memory.*, notes, shell (allowlist + sandbox Docker opt-in),
                    git, calendar, ask_user, mcp__*...
  memory/          — 3-tier (working/episodic/semantic) + Learner (background reflection)
  mcp/             — MCP CLIENT (subprocess JSON-RPC + Streamable HTTP remote) VÀ MCP SERVER (POST /mcp)
                    — xem agent-go-memory-and-mcp.md
  skills/          — progressive disclosure (list tên trong prompt, load nguyên SKILL.md khi trigger khớp)
  guardrails/      — circuit breaker, tool Kind (Read/Write/Destructive), prompt-injection filter, HITL confirm
                    — xem agent-go-resilience.md
  middleware/       — TenantMiddleware (đọc X-Tenant-ID → context), CORS
  mongo/           — driver wrapper: đọc documents/tasks(ghi)/memories(ghi)/messages(đọc)
  storage/         — sqlite (paused_runs, cost_ledger, conversation local) + chroma-style in-memory vector store
  rag/             — Voyage embedding client + Atlas vector search (Parent Doc Retrieval, HyDE, rerank)
  personality/     — formality/humor/verbosity profile
  proactive/       — cron scheduler (robfig/cron) cho prompt định kỳ
  eval/            — eval harness (exact/contains/regex/LLM-judge), wired qua CLI `jarvis eval`
  metrics/         — snapshot in-process (requests, tokens, latency)
  observability/   — slog + OpenTelemetry THẬT (stdouttrace mặc định, OTLP nếu có env) — xem agent-go-resilience.md
  config/          — đọc env, fail-fast
  transport/
    http/           — handler: /chat (SSE), /chat/resume, /mcp, /suggestions, /mcp/test-connection, /healthz, /readyz
    telegram/        — kênh Telegram long-polling — xem agent-go-channels.md
```

---

## Engine — vòng ReAct

```
        ┌────────┐   ┌───────────┐   ┌────────┐   ┌───────┐
 START ▶│ recall │──▶│ summarize │──▶│(plan)* │──▶│ model │
        └────────┘   └───────────┘   └────────┘   └───┬───┘
                                                        │ có tool_calls chưa trả lời?
                                        ┌───────────────┼────────────────┐
                                     CÓ │                                │ KHÔNG
                                        ▼                                ▼
                                   ┌────────┐                     plan còn bước? ──CÓ──▶ (reflect)* ──▶ model
                                   │ tools  │──▶ model (lặp lại)         │
                                   └────────┘                            │ KHÔNG
                                                                          ▼
                                                                     ┌─────────┐
                                                                     │ extract │──▶ END
                                                                     └─────────┘
```
*(`plan`/`reflect` chỉ chạy khi `ENABLE_PLANNING=true` — tắt mặc định để đỡ tốn 1 LLM call/lượt.)*

Sau mỗi lần chuyển node, `Engine` còn tự động **checkpoint** state vào SQLite (`paused_runs`) — xem [`agent-go-resilience.md`](./agent-go-resilience.md) để biết cơ chế resume từ bất kỳ điểm dừng nào (không chỉ khi hỏi lại user).

- **recall**: tìm memory liên quan (keyword cascade → semantic search nếu keyword không ra gì) để nạp vào system prompt.
- **summarize**: nén hội thoại nếu vượt ngân sách token (dedup tool-call trùng lặp trong batch, cộng dồn ngân sách tool-output, rồi nén thật bằng LLM — KHÔNG chèn placeholder giả khi nén lỗi).
- **model**: gọi LLM (qua provider fallback chain hoặc RouterProvider — xem [`agent-go-providers.md`](./agent-go-providers.md)), có override ngôn ngữ per-request dựa trên **ngôn ngữ câu vừa gõ** (không phải cấu hình UI tĩnh — xem `node_model.go: detectInputLanguage`, sửa sau khi phát hiện bug tự đổi ngôn ngữ giữa hội thoại). Mỗi lượt gọi cũng ghi span OTel thật (`llm.generate`) và, nếu bật, 1 dòng vào cost ledger.
- **tools**: chạy song song (`RunParallelStreaming`), dedupe lệnh gọi trùng trong cùng batch, chặn tool đặc quyền (`file.*`, `shell.exec`, `git`) nếu tenant không phải chủ hệ thống. Mỗi tool call cũng ghi span OTel (`tool.<name>`).
- **extract**: rút fact/pattern đơn giản (regex, rẻ) trước khi kết thúc lượt — khác với Learner (LLM thật, chạy nền — xem [`agent-go-memory-and-mcp.md`](./agent-go-memory-and-mcp.md)).

---

## Orchestrator — multi-agent + sticky routing

3 agent con: `general`, `code`, `research` — mỗi agent = 1 `Engine` riêng (registry tool khác nhau, `research` có thêm system prompt phụ). `route()` chọn agent theo keyword match thuần (không LLM, word-boundary cho từ ASCII đơn, substring cho cụm/tiếng Việt) — **rẻ, nhưng "mù theo lượt"**: chỉ nhìn câu hiện tại, không biết agent nào đang xử lý hội thoại.

Bug thật đã gặp: khi user trả lời câu hỏi làm rõ của `ask_user` (FE gửi lại dạng `"Q: <câu hỏi>\nA: <câu trả lời>"`), nếu câu hỏi JARVIS tự đặt ra tình cờ chứa keyword của agent khác (vd "tìm hiểu" → agent `research`), orchestrator tự route sai giữa mạch code. Fix: **sticky agent** — `Orchestrator` nhớ agent nào đang xử lý mỗi `conversationID` (map + TTL 24h, không sweep định kỳ); khi input là reply dạng `Q:/A:`, ưu tiên dùng sticky agent, bỏ qua keyword matching cho lượt đó.

Mỗi `Engine` con còn được gán 1 `agent_name` (`Engine.SetName`) — dùng để tra lại đúng Engine khi resume 1 run đã dừng (state lưu kèm `agent_name` vì mỗi agent có registry tool khác nhau, xem [`agent-go-resilience.md`](./agent-go-resilience.md)).
