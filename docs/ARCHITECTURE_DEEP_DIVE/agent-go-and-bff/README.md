# Kiến trúc agent-go & BFF — Giải thích chi tiết

> Đường chạy **chính (production)** của JARVIS: agent thật chạy ở **service Go riêng** (`services/agent-go`); Fastify (`apps/api`) chỉ còn là **BFF** (Backend-For-Frontend): auth, CRUD, upload, và **proxy** sang agent-go. LangGraph/LangChain (`apps/api/src/agent/graph.ts` + `lc-tools.ts` + `graph-runner.ts`) **vẫn còn trong repo, không bị xoá**, chạy được qua `AGENT_BACKEND=langgraph` — xem [`docs/architecture-backend-agent.md`](../../architecture-backend-agent.md) cho kiến trúc chi tiết nhánh đó, và [README § Why Not LangChain?](../../../README.md#why-not-langchain--langgraph) cho lý do viết lại bằng Go. `AGENT_BACKEND` **mặc định trong code là `"langgraph"`** (`apps/api/src/config.ts`, để dev chạy được không cần Go), nhưng **production luôn override cứng `AGENT_BACKEND=go`** (`deploy/deploy-to-vps.sh`) — mọi mô tả trong bộ tài liệu này là đường chạy `go`.

Tài liệu này được **tách thành nhiều file** để dễ đọc theo chủ đề. Đi từ trên xuống nếu đọc lần đầu:

| # | File | Nội dung |
|---|------|----------|
| 1 | (file này) | Bức tranh tổng thể, hợp đồng giữa 2 service, "3 điều đọng lại" |
| 2 | [`bff.md`](./bff.md) | Vai trò thật của BFF, danh sách route, circuit breaker phía Node |
| 3 | [`data-model.md`](./data-model.md) | MongoDB **chung** giữa BFF và agent-go — ai ghi/đọc collection nào |
| 4 | [`agent-go-core.md`](./agent-go-core.md) | Sơ đồ package, vòng ReAct (Engine), Orchestrator + sticky-agent routing |
| 5 | [`agent-go-providers.md`](./agent-go-providers.md) | Provider layer: auto-fallback chain, RouterProvider (local/cloud), pricing/cost ledger |
| 6 | [`agent-go-memory-and-mcp.md`](./agent-go-memory-and-mcp.md) | 3-tier memory + autonomous Learner; MCP — cả client (2 chế độ) lẫn server mới |
| 7 | [`agent-go-resilience.md`](./agent-go-resilience.md) | Guardrails, checkpoint/resume (crash-safe), sandbox Docker cho shell, observability thật |
| 8 | [`agent-go-channels.md`](./agent-go-channels.md) | Kênh giao tiếp: HTTP/SSE (chính), Telegram (mới) |
| 9 | [`flows.md`](./flows.md) | Trace end-to-end: chat, suggestions, resume |
| 10 | [`file-map.md`](./file-map.md) | Bảng tra nhanh khái niệm → file |

---

## 1. Bức tranh tổng thể — 2 service, 1 hợp đồng

```
 Browser                  BFF (Fastify, apps/api)              agent-go (services/agent-go)
 ───────                  ──────────────────────               ────────────────────────────
   │  cookie access_token    │                                      │
   │  POST /api/conv/:id/chat│                                      │
   ├─────────────────────────►│ authGuard: verify JWT → tenantId    │
   │                          │ appendUserMessage (Mongo)           │
   │                          │ reply.hijack() → mở SSE thô         │
   │                          │ goAgentClient.stream(history, {     │
   │                          │   tenantId, lang, attachments, ...})│
   │                          ├──────────────────────────────────────►│  X-Tenant-ID: <tenantId>
   │                          │  POST /chat  (HTTP + SSE)            │  TenantMiddleware → ctx
   │                          │◄──────────────────────────────────────┤  Orchestrator.Run(...)
   │  data: {token:"..."}  ◄──┤◄──── forward từng SSE event ─────────┤  Engine ReAct loop
   │  data: {tool_start}   ◄──┤                                       │  (xem agent-go-core.md)
   │  data: {done, usage}  ◄──┤                                       │
   │                          │ appendAssistantMessage (Mongo)        │
   │◄─────────────────────────┤ reply.raw.end()                      │
```

**Hợp đồng cốt lõi giữa 2 service**: BFF là nơi DUY NHẤT biết "user này là ai" (decode JWT cookie). Mọi request BFF gửi sang agent-go đều phải kèm header `X-Tenant-ID` — agent-go **không tự xác thực**, nó tin tưởng tuyệt đối header này (an toàn vì agent-go chỉ bind network nội bộ `jarvis`, **không publish port ra host** — xem comment trong `docker/deployment/docker-compose.yml`, service `agent-go`). Vi phạm hợp đồng này (gọi thẳng agent-go bỏ qua BFF, không kèm header) là nguồn gốc của ít nhất 1 bug thật đã xảy ra: endpoint `/suggestions` một thời gian bị FE gọi thẳng agent-go, khiến mọi user rơi về tenant `"default"` — đã fix bằng cách route qua BFF (xem [`flows.md` §2](./flows.md)).

**Kênh thứ 3 (mới) không đi qua hợp đồng này**: từ đợt sprint hardening JARVIS, agent-go còn expose thêm **Telegram** (long-polling, tự dựng tenant `telegram:<chatID>` mà không cần BFF) và **MCP server** (`POST /mcp`, không có khái niệm tenant BFF nào cả, tự có lớp auth riêng — loopback-only mặc định hoặc `MCP_API_KEY`). Cả hai đều KHÔNG đi qua BFF/JWT — xem [`agent-go-channels.md`](./agent-go-channels.md) và [`agent-go-memory-and-mcp.md`](./agent-go-memory-and-mcp.md) để biết cơ chế cô lập riêng của từng kênh.

---

## 8. Ba điều đọng lại

1. **BFF không còn là "agent" nữa** — nó là gateway xác thực + proxy. Mọi suy luận/tool/LLM nằm ở agent-go (trừ nhánh legacy LangGraph vẫn giữ song song, `AGENT_BACKEND=langgraph`).
2. **`X-Tenant-ID` là biên giới bảo mật giữa BFF và agent-go — nhưng KHÔNG PHẢI là biên giới duy nhất của agent-go nữa.** Kênh `/chat`/`/chat/resume` qua BFF vẫn tin tưởng tuyệt đối header này (chỉ BFF gọi được, localhost/network nội bộ). Nhưng Telegram và MCP server là 2 đường vào MỚI, mỗi đường tự có cơ chế cô lập/auth riêng (xem mục tương ứng) — bất kỳ endpoint mới nào thêm vào agent-go PHẢI tự hỏi rõ: đường vào này qua BFF (dùng `X-Tenant-ID`) hay là kênh độc lập (cần thiết kế auth riêng, không được ngầm định "chỉ BFF gọi được")?
3. **Mongo chung, không phải 2 DB riêng** — agent-go và BFF đọc/ghi chồng lên nhau có chủ đích (RAG documents, tasks, lịch sử hội thoại cho personalization). agent-go còn có thêm SQLite riêng (không chia sẻ với BFF) cho checkpoint/resume và cost ledger — xem [`data-model.md`](./data-model.md). Thêm collection/bảng mới cần nghĩ rõ: ai ghi, ai đọc, schema có cần định nghĩa 2 lần không.

> Xem [`README.md § Architecture`](../../../README.md#architecture) cho sơ đồ tổng quan ngắn hơn ở mức root, [`docs/architecture-backend-agent.md`](../../architecture-backend-agent.md) cho nhánh LangGraph/LangChain legacy, và [`docs/plans/`](../../plans/) cho chi tiết từng đợt fix cụ thể (routing, quota, hallucination, personalization).
