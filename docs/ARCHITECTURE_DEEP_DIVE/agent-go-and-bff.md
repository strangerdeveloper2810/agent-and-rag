# Kiến trúc agent-go & BFF — Giải thích chi tiết

> Đường chạy **chính (production)** của JARVIS từ Mốc 4: agent thật chạy ở **service Go riêng** (`services/agent-go`); Fastify (`apps/api`) chỉ còn là **BFF** (Backend-For-Frontend): auth, CRUD, upload, và **proxy** sang agent-go. LangGraph/LangChain (`apps/api/src/agent/graph.ts` + `lc-tools.ts` + `graph-runner.ts`) **vẫn còn trong repo, không bị xoá**, chạy được qua `AGENT_BACKEND=langgraph` — xem [`docs/architecture-backend-agent.md`](../architecture-backend-agent.md) cho kiến trúc chi tiết nhánh đó, và [README § Why Not LangChain?](../../README.md#why-not-langchain--langgraph) cho lý do viết lại bằng Go. `AGENT_BACKEND` **mặc định trong code là `"langgraph"`** (`apps/api/src/config.ts`, để dev chạy được không cần Go), nhưng **production luôn override cứng `AGENT_BACKEND=go`** (`deploy/deploy-to-vps.sh`) — mọi mô tả dưới đây là đường chạy `go`.

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
   │  data: {tool_start}   ◄──┤                                       │  (xem §5)
   │  data: {done, usage}  ◄──┤                                       │
   │                          │ appendAssistantMessage (Mongo)        │
   │◄─────────────────────────┤ reply.raw.end()                      │
```

**Hợp đồng cốt lõi giữa 2 service**: BFF là nơi DUY NHẤT biết "user này là ai" (decode JWT cookie). Mọi request BFF gửi sang agent-go đều phải kèm header `X-Tenant-ID` — agent-go **không tự xác thực**, nó tin tưởng tuyệt đối header này (an toàn vì agent-go chỉ bind `localhost`, không expose ra ngoài — xem `docker-compose.yml`, service `agent-go` không publish port ra host). Vi phạm hợp đồng này (gọi thẳng agent-go bỏ qua BFF, không kèm header) là nguồn gốc của ít nhất 1 bug thật đã xảy ra: endpoint `/suggestions` một thời gian bị FE gọi thẳng agent-go, khiến mọi user rơi về tenant `"default"` — đã fix bằng cách route qua BFF (xem §6.3).

---

## 2. BFF (`apps/api`) — vai trò thật, không phải "agent"

Sau khi tách agent ra Go, BFF chỉ còn 5 việc:

| Việc | Module | Ghi chú |
|---|---|---|
| **Auth** | `modules/auth`, `common/guards/auth.guard.ts` | JWT (access + refresh cookie httpOnly) + Google OAuth + OTP email (Resend). `authGuard` decode JWT → `req.tenantId = payload.sub`, set trước khi chạm controller. |
| **CRUD hội thoại** | `modules/chat` (trừ phần chat/suggestions) | `conversations`/`messages` (Mongo) — BFF **sở hữu**, ghi trực tiếp. |
| **RAG ingest** | `modules/documents` | Upload → extract text → chunk → embed (Voyage) → lưu `documents` (Mongo, có vector search Atlas). agent-go chỉ **đọc** collection này qua tool `rag.search`. |
| **File upload** | `common/storage` | MinIO/S3, dùng cho ảnh/file đính kèm chat + tài liệu RAG. |
| **Proxy sang agent-go** | `agent/client/go-agent.client.ts` | `stream()` (chat SSE), `getSuggestions()`, `testMcpConnection()`, `checkGoAgentHealth()` — MỌI lời gọi đều tự thêm `X-Tenant-ID` từ `tenantId` đã xác thực. |

BFF **không còn** business logic "agent" nào (không prompt, không tool, không LLM call trực tiếp — trừ nhánh legacy `AGENT_BACKEND=langgraph`, xem [`docs/architecture-backend-agent.md`](../architecture-backend-agent.md)). Nó là 1 lớp mỏng: xác thực → map path → gọi agent-go → forward kết quả.

### 2.1. Circuit breaker phía BFF

`go-agent.client.ts` giữ 1 circuit breaker module-level (đếm lỗi liên tiếp, mở mạch tạm thời khi agent-go down) — tách biệt hoàn toàn với circuit breaker BÊN TRONG agent-go (`guardrails.NewCircuitBreaker`, chống LLM loop) ở §5.7. Hai cơ chế bảo vệ 2 tầng khác nhau: một chống "agent-go không phản hồi được" (network/process), một chống "model tự lặp vô hạn trong 1 lượt chạy" (logic).

---

## 3. Danh sách route BFF hiện có

```
POST   /api/conversations                          tạo hội thoại
GET    /api/conversations                          danh sách
GET    /api/conversations/:id/messages              tin nhắn trong 1 hội thoại
DELETE /api/conversations/:id                       xoá hội thoại
POST   /api/conversations/:id/chat        ★         CHAT (SSE → agent-go, rate-limit 20/phút)
POST   /api/conversations/:id/continue    ★         tiếp tục câu trả lời bị cắt (cùng rate-limit)
GET    /api/suggestions                   ★         gợi ý mở hội thoại (proxy có tenant, rate-limit 20/phút)

POST   /api/documents/upload                        upload mới (.txt/.md/.pdf)
PUT    /api/documents/:documentId                   cập nhật → version mới
GET    /api/documents                               danh sách (kèm version)
DELETE /api/documents/:documentId                   xoá (cả lịch sử)

GET    /api/tasks                                   debug: xem task agent tạo (chỉ đọc)
GET    /api/health                                  health check
```

(★ = gọi LLM qua agent-go, có `preHandler: authGuard` + rate-limit riêng chặt hơn mức toàn cục.)

---

## 4. Mô hình dữ liệu — MongoDB **CHUNG** giữa BFF và agent-go

Đây là điểm dễ hiểu nhầm nhất: BFF (Node) và agent-go (Go) **không** có database riêng — cả 2 trỏ tới **cùng 1 MongoDB** (`MONGODB_URI`/`MONGODB_DB` giống nhau trong `env/.env`, cùng `docker-compose.yml`). Ai ghi/đọc collection nào:

| Collection | Ghi bởi | Đọc bởi | Ghi chú |
|---|---|---|---|
| `conversations`, `messages` | **BFF** (`chat.repository.ts`) | **BFF** (hiển thị lịch sử) + **agent-go** (đọc `RecentUserMessages` để cá nhân hoá `/suggestions`, §6.2) | agent-go chỉ ĐỌC, không bao giờ ghi 2 collection này. |
| `documents` | **BFF** (ingest: upload/update) | **agent-go** (tool `rag.search`/`rag.read`/`rag.list`) | Schema Go (`internal/mongo/models.go` struct `DocChunk`) phải tự giữ khớp tay với schema TS (Zod) — không có migration chung. |
| `document_versions` | BFF (archive khi update) | — | Chỉ BFF dùng, agent-go không chạm. |
| `tasks` | **agent-go** (qua tool CRUD) | **BFF** (`GET /api/tasks`, chỉ đọc để debug/hiển thị) | Ngược hướng với `documents`: agent-go SỞ HỮU, BFF chỉ đọc. |
| `memories` | **agent-go** (`internal/memory.Learner`) | **agent-go** | Facts đã học (autonomous learner) + knowledge items — BFF không đụng. |
| `user_mcp_servers` | **BFF** (Postgres, không phải Mongo — xem §5.6) | **agent-go** (đọc runtime khi build danh sách MCP tool remote của user) | Lưu ý: đây là **Postgres**, khác các dòng trên (Mongo) — user tự thêm remote MCP server qua Settings UI. |

**Vì sao cùng 1 Mongo**: tránh đồng bộ 2 database, và cho phép agent-go đọc trực tiếp dữ liệu BFF ghi ra (RAG documents, và giờ cả lịch sử hội thoại cho personalization) mà không cần một API nội bộ riêng để "hỏi mượn" dữ liệu. Đánh đổi: schema phải định nghĩa 2 lần (Go struct + Zod/TS interface) và tự giữ đồng bộ tay — không có migration tool chung giữa 2 ngôn ngữ.

---

## 5. agent-go (`services/agent-go`) — bộ não thật

### 5.1. Sơ đồ package

```
cmd/
  server/main.go     — HTTP entrypoint: wiring toàn bộ (provider, registry, orchestrator, learner, routes)
  jarvis/main.go      — CLI (serve / ask / chat) — chạy agent không cần HTTP

internal/
  agent/          — Engine (ReAct state machine), State, Node (model/tools/recall/summarize/extract/reflect), Router
  orchestrator/    — multi-agent: chọn agent (general/code/research) theo keyword + STICKY AGENT (§5.3) + handoff
  provider/        — LLM abstraction; factory (chọn theo env) + fallback (auto-chain) + adapter (gemini/anthropic/deepseek/ollama)
  tools/           — 25+ tool: file, web, rag.*, memory.*, notes, shell, git, calendar, ask_user, mcp__*...
  memory/          — 3-tier (working/episodic/semantic) + Learner (background reflection, §5.5)
  mcp/             — MCP client: subprocess JSON-RPC (admin, YAML) + Streamable HTTP remote (end-user, §5.6)
  skills/          — progressive disclosure (list tên trong prompt, load nguyên SKILL.md khi trigger khớp)
  guardrails/      — circuit breaker, tool Kind (Read/Write/Destructive), prompt-injection filter, HITL confirm
  middleware/       — TenantMiddleware (đọc X-Tenant-ID → context), CORS
  mongo/           — driver wrapper: đọc `documents`/`tasks`(ghi)/`memories`(ghi)/`messages`(đọc, mới)
  storage/         — sqlite (conversation local) + chroma-style in-memory vector store
  rag/             — Voyage embedding client + Atlas vector search (Parent Doc Retrieval, HyDE, rerank)
  personality/     — formality/humor/verbosity profile
  proactive/       — cron scheduler (robfig/cron) cho prompt định kỳ
  eval/            — eval harness (exact/contains/regex/LLM-judge) — thư viện, chưa wire CLI
  metrics/         — snapshot in-process (requests, tokens, latency)
  observability/   — slog + OpenTelemetry (tracer noop hiện tại)
  config/          — đọc env, fail-fast
  transport/http/   — handler: /chat (SSE), /suggestions, /mcp/test-connection, /healthz, /readyz
```

### 5.2. Engine — vòng ReAct

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

- **recall**: tìm memory liên quan (keyword cascade → semantic search nếu keyword không ra gì) để nạp vào system prompt.
- **summarize**: nén hội thoại nếu vượt ngân sách token (dedup tool-call trùng lặp trong batch, cộng dồn ngân sách tool-output, rồi nén thật bằng LLM — KHÔNG chèn placeholder giả khi nén lỗi).
- **model**: gọi LLM (qua provider fallback chain, §5.4), có override ngôn ngữ per-request dựa trên **ngôn ngữ câu vừa gõ** (không phải cấu hình UI tĩnh — xem `node_model.go: detectInputLanguage`, sửa sau khi phát hiện bug tự đổi ngôn ngữ giữa hội thoại).
- **tools**: chạy song song (`RunParallelStreaming`), dedupe lệnh gọi trùng trong cùng batch, chặn tool đặc quyền (`file.*`, `shell.exec`, `git`) nếu tenant không phải chủ hệ thống.
- **extract**: rút fact/pattern đơn giản (regex, rẻ) trước khi kết thúc lượt — khác với Learner (LLM thật, chạy nền, §5.5).

### 5.3. Orchestrator — multi-agent + sticky routing

3 agent con: `general`, `code`, `research` — mỗi agent = 1 `Engine` riêng (registry tool khác nhau, `research` có thêm system prompt phụ). `route()` chọn agent theo keyword match thuần (không LLM, word-boundary cho từ ASCII đơn, substring cho cụm/tiếng Việt) — **rẻ, nhưng "mù theo lượt"**: chỉ nhìn câu hiện tại, không biết agent nào đang xử lý hội thoại.

Bug thật đã gặp: khi user trả lời câu hỏi làm rõ của `ask_user` (FE gửi lại dạng `"Q: <câu hỏi>\nA: <câu trả lời>"`), nếu câu hỏi JARVIS tự đặt ra tình cờ chứa keyword của agent khác (vd "tìm hiểu" → agent `research`), orchestrator tự route sai giữa mạch code. Fix: **sticky agent** — `Orchestrator` nhớ agent nào đang xử lý mỗi `conversationID` (map + TTL 24h, không sweep định kỳ); khi input là reply dạng `Q:/A:`, ưu tiên dùng sticky agent, bỏ qua keyword matching cho lượt đó.

### 5.4. Provider layer — auto-fallback chain

`factory.New(cfg)` với `LLM_PROVIDER=auto` dựng chain theo thứ tự **cố định**:

```
Gemini (toàn bộ free-tier pool: primary → secondary → fallback models)
  → DeepSeek (rẻ, pay-as-you-go)
    → Claude key 1
      → Claude key 2 (optional, ANTHROPIC_API_KEY_2 — thêm sau khi 1 key hết quota giữa lúc demo)
```

`internal/provider/fallback.Provider` theo dõi cooldown/lỗi liên tiếp **theo VỊ TRÍ trong chain** (`&chain[i]`), không theo tên provider — nên 2 client Claude khác key vẫn độc lập nhau dù cùng gọi là "anthropic". Khi có 2 key, cả 2 được bọc qua `namedOverride` để log hiện `anthropic-1`/`anthropic-2` phân biệt được đang chạy key nào (chỉ áp dụng khi có key thứ 2 — 1 key thì tên vẫn là `"anthropic"` như cũ).

Riêng **reflection nền** (Learner, §5.5) dùng provider **RIÊNG** (`factory.NewReflectionProvider` — DeepSeek đơn, không qua chain Gemini) để không cạnh tranh quota Gemini với chat chính của user đang chờ trực tiếp.

### 5.5. Memory — 3 tầng + Learner nền

- **Working**: `State.Messages` trong 1 lượt chạy.
- **Episodic**: tóm tắt khi hội thoại dài (node `summarize`).
- **Semantic**: `memory.Store` (in-memory, nạp lại từ Mongo `memories` lúc khởi động) — facts sống xuyên hội thoại, theo tenant.

**Learner** chạy NGẦM sau mỗi lượt chat (`LearnFromConversation`, không block response): gọi 1 lượt LLM riêng để trích fact/knowledge item mới. Có 2 lớp giảm chi phí:
1. **Gate** `worthLearning()`: chỉ học nếu câu user có keyword gợi ý fact HOẶC đủ dài — bỏ điều kiện cũ "assistant trả lời dài" (gần như luôn đúng, làm gate vô hiệu).
2. **Batch theo N lượt** (`SetBatchTurns`, default 3 qua `REFLECTION_BATCH_TURNS`): gộp N lượt mới thực sự gọi LLM 1 lần, thay vì mỗi lượt gọi 1 lần. Cửa sổ tin nhắn đưa vào prompt co giãn theo **số lượt RAW đã trôi qua kể từ lần reflect trước** (kể cả lượt bị gate chặn) — không dùng thẳng N, để không bỏ sót nội dung khi có lượt tán gẫu xen giữa.

### 5.6. MCP — 2 chế độ, khác hẳn nhau

| | Subprocess (admin) | Remote user (Streamable HTTP) |
|---|---|---|
| Cấu hình | YAML tĩnh, chỉ admin sửa được | User tự thêm qua Settings UI ("MCP Servers" tab) |
| Lưu ở | file YAML | Postgres `user_mcp_servers` (+ `auth_token` — **plaintext tại rest**, nợ kỹ thuật đã biết) |
| Cơ chế | spawn subprocess, JSON-RPC qua stdio | HTTP POST tới URL user cung cấp, `Authorization: Bearer <token>` |
| Rủi ro | Cao (chạy lệnh trên máy chủ) → chỉ admin | Thấp hơn (chỉ gọi ra ngoài) → mọi user |
| Namespacing tool | `mcp__<server>__<tool>` | `mcp__<server>__<tool>` (cùng quy ước, tránh đụng tên) |

### 5.7. Guardrails

- **Circuit breaker** (`guardrails.NewCircuitBreaker(3)`): chặn model tự lặp vô hạn (gọi cùng 1 tool y hệt liên tiếp quá N lần).
- **Tool Kind**: mỗi tool khai `Read`/`Write`/`Destructive` — destructive cần xác nhận (HITL) trừ khi `ALLOW_DESTRUCTIVE_TOOLS=true`.
- **Prompt-injection filter**: chặn sớm input dạng "ignore all previous instructions..." trước khi mở SSE.
- **Privileged tools** (`file.*`, `shell.exec`, `git`): chỉ tenant nằm trong `OWNER_TENANT_IDS` mới gọi được — chặn ở 2 lớp (ẩn khỏi tool list VÀ chặn cứng khi model cố gọi tên tool trực tiếp).

---

## 6. Trace 2 flow thật (end-to-end)

### 6.1. Chat (`POST /api/conversations/:id/chat`)

1. BFF: `authGuard` → `tenantId`. Lưu message user vào Mongo (`messages`) TRƯỚC khi mở SSE (lỗi validate/DB còn trả JSON thường được).
2. BFF: `reply.hijack()`, mở SSE thô, gọi `goAgentClient.stream(history, {tenantId, lang, ...})`.
3. agent-go: `TenantMiddleware` đọc `X-Tenant-ID` → context. `Orchestrator.Run`: kiểm tra sticky agent (nếu là reply `ask_user`) → chọn `Engine` đúng agent.
4. `Engine`: recall → summarize → model (gọi LLM qua fallback chain) → tools (nếu có tool_calls, chạy song song) → lặp lại model → ... → extract → END. Mỗi bước phát `Event` (text/tool_start/tool_end/done) qua channel.
5. agent-go stream `Event` → BFF forward nguyên văn thành `data: {...}\n\n` → browser render real-time.
6. Sau khi response xong: BFF lưu message assistant vào Mongo; agent-go (độc lập, không chờ) spawn goroutine `Learner.LearnFromConversation` — không block response user.

### 6.2. Suggestions (`GET /api/suggestions`) — ví dụ cụ thể cho hợp đồng tenant

1. FE gọi `api.get("/api/suggestions")` (cookie session, qua BFF — **không** gọi thẳng agent-go).
2. BFF: `authGuard` → `tenantId` → `chat.controller.getSuggestions` → `go-agent.client.getSuggestions(tenantId)` → `GET {AGENT_GO_URL}/suggestions` kèm `X-Tenant-ID: <tenantId>`.
3. agent-go: đọc tenant từ context, tự query Mongo (`RecentUserMessages`, collection `messages` — đọc thứ BFF ghi) + facts (`Store.All(tenantID)`) + thời gian thật (`time.Now()`), dựng 1 prompt cá nhân hoá, gọi LLM 1 lượt (`MaxSteps: 1`), parse JSON `[{text, category}]`.
4. Trả về BFF → FE. FE lọc theo tab đang chọn ở **client-side** (không gọi lại LLM mỗi lần đổi tab — tôn trọng ngân sách LLM hẹp của dự án).

*(Trước khi fix: FE gọi thẳng agent-go, không header nào → tenant luôn `"default"`; prompt hoàn toàn tĩnh (không time/history/facts) → gợi ý quanh quẩn vài chủ đề bất kể user/thời điểm.)*

---

## 7. Bảng mapping nhanh: khái niệm → file

| Khái niệm | File |
|---|---|
| Entrypoint BFF | `apps/api/src/server.ts`, `app.ts`, `config.ts` |
| Entrypoint agent-go | `services/agent-go/cmd/server/main.go` |
| Auth + tenant identity | `apps/api/src/common/guards/auth.guard.ts` |
| Proxy BFF → agent-go | `apps/api/src/agent/client/go-agent.client.ts` |
| Tenant middleware (agent-go) | `services/agent-go/internal/middleware/tenant.go` |
| Engine ReAct loop | `services/agent-go/internal/agent/engine.go`, `node_*.go` |
| Orchestrator + sticky agent | `services/agent-go/internal/orchestrator/orchestrator.go` |
| Provider fallback chain | `services/agent-go/internal/provider/factory/factory.go`, `fallback/fallback.go` |
| Memory + Learner | `services/agent-go/internal/memory/{store,recall,learner,reflection}.go` |
| MCP (2 chế độ) | `services/agent-go/internal/mcp/`, `apps/api` Postgres `user_mcp_servers` |
| Guardrails | `services/agent-go/internal/guardrails/` |
| RAG ingest (BFF) | `apps/api/src/modules/documents/` |
| RAG tra cứu (agent-go) | `services/agent-go/internal/tools/rag.go`, `internal/rag/` |
| (Legacy) LangGraph/LangChain | `apps/api/src/agent/{graph,lc-tools,graph-runner}.ts` — xem [`docs/architecture-backend-agent.md`](../architecture-backend-agent.md) |

---

## 8. Ba điều đọng lại

1. **BFF không còn là "agent" nữa** — nó là gateway xác thực + proxy. Mọi suy luận/tool/LLM nằm ở agent-go (trừ nhánh legacy LangGraph vẫn giữ song song, `AGENT_BACKEND=langgraph`).
2. **`X-Tenant-ID` là biên giới bảo mật duy nhất giữa 2 service** — agent-go tin tưởng tuyệt đối header này vì chỉ BFF (đã qua `authGuard`) gọi được tới nó (localhost-only). Bất kỳ endpoint mới nào ở agent-go PHẢI đi qua BFF để có header này đúng cách — gọi thẳng agent-go từ FE là dấu hiệu thiết kế sai (đã xảy ra thật với `/suggestions`).
3. **Mongo chung, không phải 2 DB riêng** — agent-go và BFF đọc/ghi chồng lên nhau có chủ đích (RAG documents, tasks, giờ cả lịch sử hội thoại cho personalization). Thêm collection mới cần nghĩ rõ: ai ghi, ai đọc, schema có cần định nghĩa 2 lần không.

> Xem [`README.md § Architecture`](../../README.md#architecture) cho sơ đồ tổng quan ngắn hơn, [`docs/architecture-backend-agent.md`](../architecture-backend-agent.md) cho nhánh LangGraph/LangChain legacy, và [`docs/plans/`](../plans/) cho chi tiết từng đợt fix cụ thể (routing, quota, hallucination, personalization).
