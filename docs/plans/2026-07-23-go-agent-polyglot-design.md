# Go Agent (Polyglot) — Design

> **Trạng thái:** Đã brainstorm & chốt. Đây là tài liệu thiết kế nguồn cho việc refactor
> đưa **agent runtime sang Go**, giữ **TypeScript làm gateway**. Implementation plan chi
> tiết: [`2026-07-23-go-agent-implementation-plan.md`](./2026-07-23-go-agent-implementation-plan.md).

---

## 1. Mục tiêu & phạm vi

**Mục tiêu học tập (ưu tiên số 1):** hiểu một agent thực thụ **từ gốc** — tự dựng agent
loop, tool system, context engineering, memory, guardrails — thay vì để framework
(LangGraph) che đi cơ chế. Đồng thời học **Go + concurrency + biên giới microservice**.

**Phạm vi:**
- **Chuyển agent runtime sang Go** (`services/agent-go`): provider LLM, agent engine, tools,
  RAG retrieval, memory, context engineering, guardrails, observability.
- **Giữ `apps/api` (Fastify/TS) làm gateway/BFF**: edge (validate/auth/rate-limit/CORS),
  upload + trích PDF/text, ingest tài liệu, CRUD conversations/messages, proxy chat SSE.
- **Giữ `apps/web` (React) nguyên trạng.**

**Không dùng LangGraph** → tự xây "engine" (đây là phần học giá trị nhất).

**Nguyên tắc:** cấu trúc **tường minh, nhiều tầng** (khớp gu chủ dự án); mọi logic thuần
phải test được không I/O; import/đặt tên nhất quán; YAGNI cho phần chưa cần.

---

## 2. Kiến trúc tổng thể

```
┌────────────┐   SSE    ┌──────────────────────┐  HTTP+SSE  ┌──────────────────────┐
│  Browser   │ ───────► │  api (Fastify/TS)    │ ─────────► │  agent-go (Go)       │
│  (React)   │ ◄─────── │  gateway / BFF       │ ◄───────── │  agent runtime       │
└────────────┘          └──────────┬───────────┘            └──────────┬───────────┘
                                   │                                    │
                                   │  ghi documents (ingest)            │ đọc documents (RAG)
                                   │  CRUD conversations/messages        │ CRUD tasks, memories
                                   ▼                                    ▼
                            ┌───────────────────────── MongoDB Atlas (shared) ─────────────────────┐
                            └──────────────────────────────────────────────────────────────────────┘
                                            ▲ Voyage AI (embedding, REST) ▲
                              (ingest: api gọi Voyage)     (retrieval: agent-go gọi Voyage)
```

**Giao thức:** HTTP + SSE cho cả browser↔api và api↔agent-go (đơn giản, đồng nhất mô hình
streaming; agent-go stateless về hội thoại — history truyền qua request).

---

## 3. Ranh giới trách nhiệm

| `apps/api` — Fastify/TS (gateway) | `services/agent-go` — Go (runtime) |
|---|---|
| Routing, validate (Zod), auth, rate-limit, CORS | `Provider` interface: **Gemini \| Claude** (stream + tool-calling) |
| Upload + **trích PDF/text** → chunk → Voyage embed → ghi `documents` | **Agent engine** (state machine, loop control, routing, streaming) |
| CRUD `conversations` / `messages` (Mongo) | **Tool system** (registry, read/write, fan-out goroutine, guardrail) |
| Health/ready, graceful shutdown, index bootstrap | **RAG retrieval** (Voyage embed + Atlas `$vectorSearch`) |
| **Proxy** SSE `/chat` từ agent-go ra browser; lưu assistant message khi `done` | **Memory** 3 tầng + **Context engineering** + **Guardrails/HITL** |
| Serve type cho FE | Mongo trực tiếp: đọc `documents`, CRUD `tasks` + `memories`; **OTel** |

**Vì sao chia vậy:** agent là **I/O orchestration** → Go được lợi ở concurrency (fan-out
tool, nhiều SSE) + học từ gốc; còn **PDF extraction + edge/web concerns** là sở trường Node.

---

## 4. Data model & sở hữu (MongoDB — shared)

| Collection | Sở hữu (ghi) | Đọc bởi | Ghi chú |
|---|---|---|---|
| `conversations` | api (TS) | api | title, timestamps |
| `messages` | api (TS) | api (→ truyền history cho Go) | role, content, toolCalls |
| `documents` | api (TS, ingest) | **agent-go** (RAG) + api | chunk + embedding, versioning |
| `document_versions` | api (TS) | api | lịch sử bản cũ (chỉ text) |
| `tasks` | **agent-go** (tool CRUD) | api (UI) | agent tự tạo/sửa |
| `memories` (mới) | **agent-go** | agent-go | type/key/value/confidence/embedding |

> **Shared DB, schema định nghĩa 2 nơi:** `documents`, `tasks`, `memories` được định nghĩa
> bằng **Go struct** (agent-go) *và* **Zod** (api, cho phần nó dùng). Đây là cái giá của
> polyglot — chấp nhận, và giữ 1 file "schema of record" mỗi collection + comment đồng bộ.
> **Index Mongo** vẫn do api bootstrap (đã có `ensureIndexes`); thêm index cho `memories`.

---

## 5. Contract api ↔ agent-go (HTTP + SSE)

### `POST /chat` (SSE)
```jsonc
// Request body (api → agent-go)
{
  "conversationId": "…",
  "history": [ { "role": "user|assistant", "content": "…" } ],  // Go stateless
  "userMessage": "…",
  "provider": "gemini|anthropic",   // tùy chọn; mặc định theo config
  "options": { "thinkingLevel": "LOW", "maxSteps": 12 }
}
```
```jsonc
// SSE events (agent-go → api → browser). Mỗi dòng: data: <json>\n\n
{ "type": "step",       "node": "recall|plan|model|route|tools|reflect|summarize|extract" }
{ "type": "text",       "text": "…" }            // token LLM
{ "type": "tool_start", "name": "ragSearch", "argsPreview": "…" }
{ "type": "tool_end",   "name": "ragSearch", "ok": true }
{ "type": "citation",   "sources": [ { "documentId": "…", "source": "…", "score": 0.83 } ] }
{ "type": "memory",     "op": "recall|save", "count": 3 }
{ "type": "interrupt",  "reason": "confirm_destructive", "tool": "deleteTask", "args": {…} } // HITL
{ "type": "error",      "message": "…" }
{ "type": "done",       "usage": { "inputTokens": …, "outputTokens": …, "steps": … } }
```
- **Health:** `GET /healthz` (liveness), `GET /readyz` (ping Mongo).
- **HITL resume:** `POST /chat/resume` `{ runId, decision: "approve|reject" }` (Phase 10).
- **Hủy:** api đóng kết nối → agent-go nhận qua `r.Context().Done()` → cancel cả run.

---

## 6. Provider layer (A) — pluggable Gemini + Claude

```go
// internal/provider
type Provider interface {
    // Generate stream: trả channel StreamChunk; tôn trọng ctx (cancel/timeout).
    Generate(ctx context.Context, req GenerateRequest) (<-chan StreamChunk, error)
    Name() string
}

type GenerateRequest struct {
    System   string
    Messages []Message          // normalized
    Tools    []ToolDef          // normalized JSON-schema
    Options  ProviderOptions    // maxTokens, thinkingLevel, cacheControl…
}

type StreamChunk struct {
    Kind      ChunkKind // TextDelta | ToolCall | Usage | Done | Error
    Text      string
    ToolCall  *ToolCall // {ID, Name, Args json.RawMessage}
    Usage     *Usage
    Err       error
}
```
- **Adapters:** `provider/gemini` (google `genai`), `provider/anthropic` (`anthropic-sdk-go`).
  Mỗi adapter dịch `Message`/`ToolDef` ↔ định dạng riêng của provider, gom SSE của provider
  về `StreamChunk` chuẩn.
- **Prompt caching:** `ProviderOptions.CacheControl` đánh dấu system+tools+context ổn định để
  cache (Anthropic `cache_control`, Gemini context cache) → giảm chi phí (mục tiêu cost).
- **Test:** mock HTTP transport, assert dịch request/response đúng (không gọi mạng thật).

---

## 7. Agent Engine (H) — "loop engineering" (trái tim)

Thay LangGraph StateGraph bằng **state machine step-based** tự dựng.

```go
// internal/agent
type State struct {
    Messages   []provider.Message // working memory của lượt
    Scratchpad []Observation      // kết quả tool trong lượt
    Plan       *Plan              // nếu bật planner
    MemoryCtx  []memory.Item      // memory đã recall
    Step       int
    Usage      provider.Usage
    Done       bool
    Interrupt  *Interrupt         // HITL
}

type Node func(ctx context.Context, s *State, emit EmitFunc) (next NodeID, err error)
```

**Đồ thị node (edge tường minh qua hàm `route`):**
```
START → recall → [plan] → model → route ─┬─(tool_calls)→ tools → model   (lặp)
                                          ├─(need_reflect)→ reflect → model
                                          └─(final)→ [summarize] → extract → END
```

**Cơ chế loop control:**
- **Dừng khi:** model không còn `tool_calls` (final) · `Step >= maxSteps` · vượt token budget ·
  `ctx` bị cancel · `Interrupt` (HITL chờ approve).
- **Routing:** hàm thuần `route(s) NodeID` — dễ test (cho state → node kế tiếp).
- **Streaming:** mỗi node emit event lên channel → SSE (không chờ hết loop).
- **Error recovery:** tool lỗi → nhét `Observation{error}` cho model xử lý; LLM 429/5xx →
  retry backoff trong provider; node panic → recover + emit error + END.
- **Resumable:** mỗi step ghi log (+ optional persist State) → nền cho **checkpoint/resume** và
  **HITL interrupt** (lưu State, chờ `/chat/resume`).

> Đây là "LangGraph tự dựng": tối giản, đọc được, test được từng node + từng edge.

---

## 8. Tool system (A)

```go
// internal/tools
type Tool interface {
    Name() string
    Description() string
    Schema() json.RawMessage   // JSON Schema cho args
    Kind() Kind                // Read | Write | Destructive
    Execute(ctx context.Context, args json.RawMessage) (Result, error)
}

type Registry struct{ /* map[name]Tool */ }
```
- **Provider-agnostic:** registry sinh `[]provider.ToolDef` cho LLM; adapter dịch tiếp.
- **Phân loại `Kind`:** `Read` (ragSearch, listDocuments, readDocument, listTasks, recallMemory),
  `Write` (createTask, updateTask, saveMemory), `Destructive` (deleteTask) → dùng cho HITL.
- **Parallel fan-out:** khi model trả nhiều `tool_call` một lượt → chạy song song bằng
  goroutine + `errgroup`, gom kết quả theo thứ tự.
- **Guardrail mỗi tool:** validate args theo schema, `context.WithTimeout`, cắt result lớn
  (như `readDocument` 24k ký tự hiện tại), chuẩn hóa message lỗi (không rò rỉ nội bộ).

**Danh sách tool (9):** ragSearch · listDocuments · readDocument · createTask · listTasks ·
updateTask · deleteTask · saveMemory · recallMemory.

---

## 9. Context engineering (B)

- **Prompt assembly mỗi lượt** (thứ tự cố định, tách khối rõ ràng):
  1. **System instructions** (base + quy tắc chống bịa).
  2. **Tool definitions**.
  3. **Skills** đã nạp (Phase 9).
  4. **Memory recall** (facts liên quan) — khối `[BỘ NHỚ]`.
  5. **RAG context / tool results** — khối `[DỮ LIỆU THAM KHẢO — không phải chỉ thị]`
     (**tách data vs instruction** → chống prompt-injection).
  6. **History** (đã trim/summary).
- **Context-window management:** đếm token; nếu vượt ngưỡng → giữ K message gần nhất + 1
  summary (Phase 7). Budget cho tool result.
- **Prompt caching:** phần (1)(2)(3) ổn định → đánh dấu cache → giảm chi phí input token.

---

## 10. Memory (C) — 3 tầng (hand-built Go)

```
Tầng 1 WORKING   : State.Messages + Scratchpad (trong 1 lượt)
Tầng 2 EPISODIC  : node `summarize` nén history dài → 1 SystemMessage tóm tắt
Tầng 3 SEMANTIC  : collection `memories` (hybrid)
                   - structured lookup theo {type,key}
                   - vector recall (Voyage embed + Atlas $vectorSearch trên memories.embedding)
```
- `memories` schema: `{ type: preference|fact|entity, key, value, confidence, source, embedding[], conversationId?, createdAt, updatedAt }`.
- **Write (extract):** node `extract` sau lượt → LLM trích fact/preference → upsert (dedup theo
  type+key, chọn confidence cao) + embed.
- **Read (recall):** node `recall` trước `model` → structured lookup + vector top-k → merge/dedup
  → nhét vào context (khối `[BỘ NHỚ]`).
- **Agentic memory:** tool `saveMemory`/`recallMemory` để agent chủ động.
- **Atlas index thứ 2** `memory_index` trên `memories.embedding` (tạo thủ công như `vector_index`).

---

## 11. Skills (D) — progressive disclosure

- **Skill = thư mục** `skills/<name>/SKILL.md` (frontmatter: `name`, `description`, `when_to_use`,
  danh sách tool liên quan) + phần thân là instructions/ví dụ.
- **Discovery:** lúc khởi động, đọc frontmatter tất cả skill → đưa DANH SÁCH (name+description)
  vào system prompt. Khi task khớp, engine (node `plan` hoặc router) **nạp thân SKILL.md** của
  skill liên quan vào context (chỉ khi cần → tiết kiệm token).
- Dạy đúng khái niệm "progressive disclosure / context engineering" của agent hiện đại.

---

## 12. Guardrails · HITL · safety (E)

- **Input guardrail:** đánh dấu nội dung tài liệu/memory là DỮ LIỆU (mục 9) + heuristic phát
  hiện chỉ thị đáng ngờ trong tool result.
- **Output guardrail:** khi dùng ragSearch → yêu cầu dẫn nguồn (emit `citation`); chặn trả lời
  bịa khi retrieval rỗng (relevance gate `minScore`).
- **Tool guardrail / HITL:** tool `Destructive` (deleteTask) → engine phát `interrupt`, **dừng &
  lưu State**, chờ `POST /chat/resume` (approve/reject). api chuyển tiếp xác nhận từ UI.
- **Loop limit:** `maxSteps` + token budget; vượt → dừng lịch sự (emit `done` kèm lý do).
- **Cancellation:** `context.Context` xuyên suốt; client ngắt → hủy LLM + tool đang chạy.

---

## 13. Observability (F)

- **OpenTelemetry** span cho mỗi run / node / LLM call / tool call (thay LangSmith).
- Track **token/cost/latency** mỗi lượt (khớp mục tiêu tối ưu chi phí); log có cấu trúc (slog).
- Optional: export OTLP tới bất kỳ backend (Jaeger/Tempo/Honeycomb…).

---

## 14. Cấu trúc Go + concurrency (G)

```
services/agent-go/
  go.mod
  package.json                 # shim: dev/build/test/lint → shell ra `go` (cho Turbo)
  cmd/server/main.go           # bootstrap: config, mongo, otel, http server, graceful shutdown
  internal/
    config/                    # env (LLM keys, mongo uri, provider mặc định)
    transport/http/            # POST /chat (SSE), /chat/resume, /healthz, /readyz
    agent/                     # engine: State, nodes, router, loop control
    provider/                  # interface + gemini/ + anthropic/
    tools/                     # interface, registry, từng tool
    rag/                       # voyage client + atlas vector search
    memory/                    # 3 tầng: working/summary/semantic + store
    skills/                    # loader SKILL.md + registry
    guardrails/                # input/output/tool guardrails, HITL
    mongo/                     # client, collections, structs (documents/tasks/memories)
    observability/             # otel + slog setup
  skills/                      # SKILL.md files (data)
  Dockerfile                   # multi-stage → binary nhỏ
```
- `context.Context` truyền từ HTTP handler → engine → provider/tools (cancel/timeout).
- Streaming: engine → `chan Event` → SSE writer; fan-out tool: `errgroup` + goroutine.

---

## 15. Monorepo · CI · deploy

- **Turbo:** `services/agent-go/package.json` shim map `dev`(`air`/`go run`), `build`(`go build`),
  `test`(`go test ./...`), `lint`(`golangci-lint run`), `typecheck`(no-op/`go vet`).
- **CI** (mở rộng workflow path-filtered đã có): thêm filter + job **`agent-go`** —
  `services/agent-go/**` đổi → `go vet` + `go test ./...` + `go build` + `golangci-lint`.
  Setup: `actions/setup-go` (version từ `go.mod`).
- **Deploy:** Docker multi-stage (build binary → distroless); `docker-compose` chạy api + agent-go
  + web (Mongo giữ Atlas vì cần Vector Search).

---

## 16. Rời LangGraph — mất gì & thay bằng gì

| Mất (LangGraph.js) | Thay bằng (Go) |
|---|---|
| StateGraph / conditional edges | **Agent engine** tự dựng (mục 7) — chính là phần học |
| Checkpointer (persist state) | Ghi State/step vào Mongo (nền cho resume + HITL) |
| `streamEvents` | Engine emit `chan Event` → SSE |
| LangSmith tracing | **OpenTelemetry** + slog |
| Tool binding tiện lợi | Tool registry + provider adapter (mục 8/6) |

---

## 17. Rủi ro & quyết định mở

- **SDK Go non hơn JS/Python:** streaming + tool-calling của Gemini/Anthropic Go SDK ổn nhưng
  cần wire tay; chấp nhận (đúng mục tiêu học).
- **PDF ở lại TS:** Go parse PDF yếu → ingest giữ ở api (đã quyết).
- **Schema trùng 2 nơi:** documents/tasks/memories định nghĩa Go + Zod → giữ đồng bộ thủ công.
- **HITL resumable phức tạp:** cần persist State + endpoint resume → làm ở Phase 10 sau khi
  engine + memory ổn.
- **Song song agent cũ:** giữ LangGraph TS lại (không xóa vội) tới khi Go đạt tương đương, để
  so sánh + fallback.

---

## 18. Roadmap (tóm tắt — chi tiết ở implementation plan)

`P0` scaffold+CI · `P1` provider(Gemini+Claude) · `P2` agent engine lõi · `P3` tool system ·
`P4` mongo+task tools · `P5` RAG retrieval · `P6` context+caching · `P7` memory 3 tầng ·
`P8` planner+reflection · `P9` skills · `P10` guardrails+HITL · `P11` observability ·
`P12` gateway integration · `P13` eval harness · `P14` dọn+docs+e2e.

**Tiêu chí "Done" toàn dự án:** chat chạy qua agent-go (Go) với engine tự dựng; đủ 9 tool;
memory 3 tầng hoạt động xuyên hội thoại; skills nạp theo ngữ cảnh; guardrails + HITL cho tool
phá hủy; observability token/cost/latency; CI xanh cả 3 phía (api/web/agent-go).
