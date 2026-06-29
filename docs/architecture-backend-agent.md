# Kiến trúc Backend & Agent — Giải thích chi tiết

> Tài liệu này mô tả **trạng thái hiện tại** của repo (sau Mốc 3 — LangGraph).
> Mục tiêu: hiểu rõ từng tầng backend, agent core, và **flow chạy** của một request.

---

## 1. Bức tranh tổng thể

Backend là một API **Fastify** (Node + TypeScript, ESM) tổ chức theo **module hoá theo tính năng**, layering chuẩn **routes → controller → service → repository**, cộng một tầng **middleware** (error handler tập trung):

```
  middleware            routes            controller          service             repository        lib
  ──────────            ──────            ──────────          ───────             ──────────        ───
  setErrorHandler   →   map path →    →   xử lý HTTP:      →  business logic:  →  Mongo queries  →  mongo,
  (map lỗi→status)      controller        parse req/reply     orchestration        (1 collection)    voyage…
                                          gọi service         (SSE, ingest…)
```

Quy tắc: **mỗi tầng chỉ biết tầng ngay dưới**, và controller chỉ `throw` lỗi — middleware lo map sang HTTP status. Quy ước này lặp lại ở mọi module (`chat`, `documents`, `tasks`).

**Sơ đồ thư mục `apps/api/src`:**

```
src/
├── server.ts              # entrypoint: connect Mongo → listen
├── app.ts                 # dựng Fastify, đăng ký middleware + plugin + routes
├── config.ts              # đọc & validate env (Zod), fail-fast
│
├── middleware/            # tầng cross-cutting (Fastify hooks)
│   └── error-handler.ts   #   setErrorHandler tập trung: lỗi có kiểu → HTTP status
│
├── lib/                   # hạ tầng dùng chung (nói chuyện với thế giới ngoài)
│   ├── mongo.ts           #   kết nối Mongo (singleton)
│   ├── claude.ts          #   Anthropic SDK client  (dùng ở agent legacy)
│   ├── voyage.ts          #   Voyage embedding client + retry 429
│   └── errors.ts          #   HttpError / BadRequestError / NotFoundError
│
├── schemas/               # Zod schema dùng chung (validate + type)
│   ├── message.ts  task.ts  conversation.ts
│
├── modules/               # mỗi tính năng tách tầng vào sub-folder + barrel index.ts
│   ├── chat/              #   hội thoại + endpoint chat (SSE → agent)
│   │   ├── controllers/   #     chat.controller.ts + index.ts (barrel)
│   │   ├── services/      #     chat.service.ts + index.ts
│   │   ├── repositories/  #     chat.repository.ts + index.ts
│   │   ├── chat.routes.ts #     map path → controller
│   │   └── index.ts       #     barrel module: export { chatRoutes }
│   ├── documents/         #   RAG (+ extract.ts, chunk.ts là helper ở gốc module)
│   │   ├── controllers/  services/  repositories/
│   │   ├── extract.ts  chunk.ts     # helper text-extract / chunking
│   │   ├── documents.routes.ts  index.ts
│   └── tasks/             #   CRUD task (controllers/ services/ repositories/ + routes + index)
│
├── agent/                 # AGENT CORE
│   ├── graph.ts           #   [ACTIVE] LangGraph StateGraph
│   ├── lc-tools.ts        #   [ACTIVE] 7 tool dạng LangChain
│   ├── graph-runner.ts    #   [ACTIVE] chạy graph + map event → SSE
│   ├── print-graph.ts     #   in sơ đồ Mermaid (học/visualize)
│   ├── tools.ts           #   [LEGACY] tool dạng Anthropic SDK (Mốc 2)
│   └── agent-loop.ts      #   [LEGACY] vòng while thủ công (Mốc 2)
│
└── scripts/
    └── backfill-document-versions.ts   # migration documentId/version
```

> **ACTIVE vs LEGACY:** Sau Mốc 3, đường chạy agent là **LangGraph** (`graph.ts` + `lc-tools.ts` + `graph-runner.ts`). Hai file `agent-loop.ts` (vòng `while`) và `tools.ts` (tool kiểu Anthropic) là **bản cũ Mốc 2 giữ lại để học/so sánh**, KHÔNG còn được gọi (`chat.service` giờ gọi thẳng `graph-runner`).

---

## 2. Vòng đời một request chat (end-to-end)

Đây là flow quan trọng nhất — đọc kỹ phần này là hiểu cả hệ thống.

```
Trình duyệt                Fastify route              LangGraph agent           Mongo / API ngoài
───────────                ─────────────              ───────────────           ─────────────────
  │ POST /conversations/:id/chat {content}
  ├──────────────────────────►│
  │                           │ addMessage(user)  ────────────────────────────────► messages (Mongo)
  │                           │ getMessages → history
  │                           │ mở SSE (reply.raw)
  │                           │ runGraph(history)
  │                           ├────────────────────────►│ streamEvents(v2)
  │                           │                          │
  │                           │                          │  node "agent":
  │                           │                          │   ChatAnthropic.invoke ──► Anthropic API
  │   data:{token:"..."}  ◄───┤◄── on_chat_model_stream ─┤   (stream từng token)
  │   data:{token:"..."}  ◄───┤                          │
  │   data:{tool_start}   ◄───┤◄── on_tool_start ────────┤  node "tools":
  │                           │                          │   ragSearch/readDocument… ─► Voyage + Mongo
  │   data:{tool_end}     ◄───┤◄── on_tool_end ──────────┤
  │                           │                          │  (quay lại node "agent" — lặp)
  │   data:{token:"..."}  ◄───┤◄── on_chat_model_stream ─┤   ChatAnthropic.invoke (lần 2…)
  │   data:{done:true}    ◄───┤   (graph END)            │
  │                           │ addMessage(assistant, full) ─────────────────────► messages (Mongo)
  │◄──────────────────────────┤ reply.raw.end()
```

**Tóm tắt 6 bước:**
1. Route nhận `content`, **lưu message user** vào Mongo, lấy lại lịch sử hội thoại.
2. Mở **SSE** (Server-Sent Events) bằng `reply.raw` thô.
3. Gọi `runGraph(history)` — async generator.
4. LangGraph chạy vòng `agent ↔ tools`, **phát event**: token text, tool_start/end.
5. Route map mỗi event thành 1 dòng `data: {...}\n\n` đẩy về trình duyệt **real-time**.
6. Stream xong → **lưu câu trả lời** assistant vào Mongo → đóng SSE.

File: [`modules/chat/chat.routes.ts`](../apps/api/src/modules/chat/chat.routes.ts) — endpoint `POST /conversations/:id/chat`.

---

## 3. Backend chi tiết

### 3.1. Khởi động & cấu hình

| File | Vai trò |
|---|---|
| `server.ts` | Entrypoint: `connectMongo()` → `app.listen()`. Crash sớm nếu Mongo lỗi. |
| `app.ts` | `buildApp()`: dựng Fastify, đăng ký `cors`, `multipart` (limit 25MB cho PDF), rồi `register` 3 module dưới prefix `/api`. Tách hàm để test gọi được mà không mở port. |
| `config.ts` | Đọc env qua **Zod** và `parse` **ngay lúc import** → thiếu key là crash với message rõ ràng (fail-fast). Biến lạ (vd `LANGSMITH_*`) được Zod bỏ qua. |

### 3.2. Tầng hạ tầng — `lib/`

Nơi "nói chuyện với thế giới ngoài", viết một lần dùng mọi nơi:

- **`mongo.ts`** — giữ **một** kết nối Mongo dạng singleton. `connectMongo()` chỉ connect lần đầu; `getDb()` trả về db đã kết nối (ném lỗi nếu chưa). Tránh mở connection mỗi request.
- **`claude.ts`** — khởi tạo SDK Anthropic + export `CLAUDE_MODEL` (mặc định Haiku 4.5). *Dùng ở agent legacy + chat.service legacy.* Agent active (LangGraph) dùng `ChatAnthropic` của LangChain riêng.
- **`voyage.ts`** — gọi Voyage AI để **embed** text → vector 1024 chiều. Hai điểm cốt lõi:
  - Phân biệt `input_type: "document"` (lúc nạp) vs `"query"` (lúc tìm) — Voyage tối ưu vector khác nhau, giúp search chính xác hơn.
  - **Retry với backoff** khi gặp 429 (free tier 3 req/phút), bọc lỗi trong `VoyageError` mang `status`.

### 3.3. Module pattern (mỗi tầng một sub-folder + barrel)

Mỗi module tách tầng vào **sub-folder riêng**, mỗi folder có `index.ts` (barrel) re-export. Lấy `chat`:

| Đường dẫn | Tầng | Việc |
|---|---|---|
| `chat.routes.ts` | routes | Chỉ map đường dẫn → controller (thin, không logic). |
| `controllers/chat.controller.ts` | controller | Xử lý HTTP: đọc params/body, gọi service, trả/stream response. Endpoint chat mở SSE và pipe event từ service. |
| `services/chat.service.ts` | service | Business logic: orchestrate agent (`streamReply`), CRUD hội thoại, lưu message. **Không** chạm `req/reply`. |
| `repositories/chat.repository.ts` | repository | Chỉ thao tác Mongo: conversation + message. |
| `index.ts` (mỗi folder + module) | barrel | Re-export để import gọn: `from "../services"` thay vì `from "../services/chat.service"`. |

**Luồng import qua barrel:** `routes → "./controllers"` → controller `→ "../services"` → service `→ "../repositories"`. Module root `index.ts` export `chatRoutes` để `app.ts` chỉ cần `from "./modules/chat"`.

> `documents` có thêm 2 helper ở gốc module: `extract.ts` (file→text, đa định dạng) và `chunk.ts` (cắt chunk) — dùng bởi service, không thuộc 3 tầng nên để ngoài sub-folder.

> **Tách routes vs controller:** trong Fastify, route file đăng ký path; controller là các hàm `(req, reply)` xử lý. Service nhận tham số thường (không biết HTTP) → dễ test, dễ tái dùng. Controller chỉ `throw` lỗi → [`middleware/error-handler.ts`](../apps/api/src/middleware/error-handler.ts) map sang status (415/422/429/413/400/404/500), không try/catch rải rác.

**Các route hiện có:**
```
POST   /api/conversations                      tạo hội thoại
GET    /api/conversations                      danh sách
GET    /api/conversations/:id/messages         tin nhắn trong 1 hội thoại
DELETE /api/conversations/:id                  xoá hội thoại
POST   /api/conversations/:id/chat             ★ CHAT (SSE → LangGraph agent)

POST   /api/documents/upload                   upload mới (.txt/.md/.pdf)
PUT    /api/documents/:documentId              cập nhật → version mới
GET    /api/documents                          danh sách (kèm version)
GET    /api/documents/:documentId/versions     lịch sử version
GET    /api/documents/:id/versions/:version    nội dung 1 version
DELETE /api/documents/:documentId              xoá (cả lịch sử)

GET    /api/tasks                              debug: xem task agent tạo
GET    /api/health                             health check
```

### 3.4. Module `tasks`

- `tasks.repository.ts` — CRUD trên collection `tasks`: `createTask` (đánh dấu `source: "user"|"agent"`), `listTasks` (lọc status/priority/tag), `updateTask` (đặt `completedAt` khi status=done), `deleteTask`.
- `tasks.routes.ts` — chỉ có 1 route debug `GET /tasks` để quan sát task agent tạo.
- Task **chủ yếu được tạo qua agent tool**, không phải user gọi REST trực tiếp.

---

## 4. RAG pipeline (module `documents`)

RAG = cách agent **tra cứu nội dung tài liệu**. Có 2 thời điểm:

### 4.1. Lúc UPLOAD (ingest)

```
file (.txt/.md/.pdf)
  │  documents.routes: POST /documents/upload
  ▼
extractDocumentText(filename, buffer)          ← extract.ts (dispatch theo đuôi)
  │   .txt/.md → buffer.toString("utf-8")
  │   .pdf     → unpdf (pdfjs) trích text
  │   khác     → UnsupportedFileError (415)
  │   rỗng     → EmptyContentError (422, vd PDF scan)
  ▼
chunkText(content)                             ← chunk.ts (RecursiveCharacterTextSplitter, 800/overlap 100)
  ▼
embed(chunks, "document")                      ← voyage.ts → vector[1024] cho mỗi chunk
  ▼
buildChunkDocs(documentId, source, version, …) ← documents.service.ts (hàm thuần, có test)
  ▼
insertChunks(docs)                             ← lưu vào collection "documents"
```

Mỗi chunk được embed **một lần** rồi lưu cố định. Upload mới sinh `documentId` mới, `version = 1`.

### 4.2. Lúc agent TRA CỨU (search)

Tool `ragSearch` (trong `lc-tools.ts`):
```
query
  ▼ embed([query], "query")        ← voyage.ts
  ▼ searchSimilar(vec, 5)          ← documents.repository.ts
       └─ Atlas $vectorSearch (index "vector_index", cosine, numCandidates 100)
  ▼ trả 5 chunk gần nghĩa nhất + score
```

### 4.3. Versioning (cập nhật tài liệu)

Bài toán: upload bản mới không được trộn lẫn chunk cũ. Giải bằng **2 collection**:

```
documents          ← chỉ chứa bản MỚI NHẤT (có embedding → search như cũ)
{ documentId, source, version, chunkIndex, text, embedding, createdAt }

document_versions  ← lưu bản CŨ (chỉ text, KHÔNG embedding → đỡ tốn Voyage)
{ documentId, version, source, content, archivedAt }
```

Luồng `PUT /documents/:documentId` (`updateDocument`):
```
1. archiveCurrentVersion(documentId)   → snapshot bản hiện tại sang document_versions, xoá khỏi documents
2. chunk + embed nội dung mới
3. insert với version = cũ + 1
```
→ `documents` **luôn chỉ có bản mới nhất** nên `searchSimilar` và Atlas index **không phải đổi**.

> **Cap readDocument:** `getDocumentContent` cắt nội dung ở **24k ký tự**. Tài liệu lớn (PDF mấy trăm trang) trả nguyên văn sẽ tốn ~150k token mỗi lượt → chậm/đắt. Khi bị cắt, trả `note` nhắc agent dùng `ragSearch`.

---

## 5. Agent core (LangGraph) ★

Đây là phần "bộ não". Sau Mốc 3, agent = một **StateGraph** (state machine), thay cho vòng `while` thủ công.

### 5.1. Ba file & vai trò

| File | Vai trò |
|---|---|
| `lc-tools.ts` | Bọc **7 tool** thành LangChain `tool()`: `ragSearch`, `listDocuments`, `readDocument`, `createTask`, `listTasks`, `updateTask`, `deleteTask`. Mỗi tool = async fn + `{name, description, schema (Zod)}`. |
| `graph.ts` | Định nghĩa `StateGraph`: state = `messages`; node `agent` (gọi Claude đã bind tool) + node `tools` (`ToolNode` chạy tool); cạnh điều kiện. |
| `graph-runner.ts` | `runGraph(history)`: chạy `streamEvents(v2)`, **map** event LangGraph → `AgentEvent` (`text`/`tool_start`/`tool_end`) cho SSE. |

### 5.2. Sơ đồ StateGraph

```
            ┌─────────┐
            │  START  │
            └────┬────┘
                 │   addEdge(START, "agent")
                 ▼
        ┌───────────────────┐
   ┌───▶│       agent        │   ChatAnthropic.bindTools(lcTools).invoke
   │    │   (agentNode)      │   → đẻ ra 1 AIMessage
   │    └─────────┬─────────┘
   │              │  shouldContinue(state):
   │              │  "message cuối có tool_calls không?"
   │         ┌────┴─────┐
   │     CÓ  │          │  KHÔNG
   │         ▼          ▼
   │    ┌─────────┐  ┌─────────┐
   │    │  tools   │  │   END   │  → trả lời cho user
   │    │(ToolNode)│  └─────────┘
   │    └────┬────┘
   │         │   addEdge("tools", "agent")
   └─────────┘   (luôn quay lại agent sau khi chạy tool)
```

**Mapping về [`graph.ts`](../apps/api/src/agent/graph.ts):**

| Trong sơ đồ | Code | Vai trò |
|---|---|---|
| node `agent` | `agentNode` | Bơm `SystemMessage` (prompt hardened) + `state.messages` vào Claude, trả message mới |
| node `tools` | `new ToolNode(lcTools)` | Chạy tool Claude yêu cầu, trả `ToolMessage` |
| `START → agent` | `.addEdge(START, "agent")` | Vào graph chạy agent trước |
| nhánh CÓ/KHÔNG | `.addConditionalEdges("agent", shouldContinue, …)` | Rẽ dựa trên `shouldContinue` |
| điều kiện | `shouldContinue` | `last.tool_calls?.length ? "tools" : END` |
| `tools → agent` | `.addEdge("tools", "agent")` | Quay lại để Claude đọc kết quả tool |
| state | `MessagesAnnotation` | State = mảng message, **tự cộng dồn** (reducer) qua mỗi node |

### 5.3. Trace một ví dụ multi-step

Câu: *"Có những tài liệu nào? Đọc giúp tôi một cái."*

```
messages: [Human: "có những tài liệu nào? đọc giúp…"]
  ▼ agent  → Claude: cần listDocuments
messages += [AI: tool_calls=[listDocuments]]
  ▼ shouldContinue → "tools"
  ▼ tools  → listDocuments → "[{source:..., chunks:882}, …]"
messages += [Tool: …]
  ▼ agent  → Claude: đọc 1 cái → readDocument
messages += [AI: tool_calls=[readDocument]]
  ▼ shouldContinue → "tools"
  ▼ tools  → readDocument (đã cap 24k)
messages += [Tool: …]
  ▼ agent  → Claude viết câu trả lời (không gọi tool)
messages += [AI: "Có 3 tài liệu: …"]
  ▼ shouldContinue → END
```

Vòng `agent → tools → agent → tools → agent` lặp đến khi Claude thôi gọi tool.
`state.messages` **lớn dần** mỗi vòng → đó là "bộ nhớ làm việc" của một lượt, và lý do token tăng dần (xem trace LangSmith).

### 5.4. streamEvents → SSE

`runGraph` dùng `agentGraph.streamEvents({messages}, {version:"v2"})`. LangGraph phát event chi tiết; `mapGraphEvent` lọc 3 loại:

| Event LangGraph | → AgentEvent | → SSE gửi browser |
|---|---|---|
| `on_chat_model_stream` (có text delta) | `{type:"text", text}` | `data:{token:"…"}` |
| `on_tool_start` | `{type:"tool_start", name}` | `data:{type:"tool_start",name}` |
| `on_tool_end` | `{type:"tool_end", name}` | `data:{type:"tool_end",name}` |
| (còn lại) | `null` (bỏ qua) | — |

→ Frontend không cần đổi gì khi chuyển từ Mốc 2 sang Mốc 3 vì **format `AgentEvent` giữ nguyên**.

### 5.5. System prompt (grounding)

`graph.ts` dùng prompt đã **hardened** chống bịa đặt:
- "TUYỆT ĐỐI KHÔNG bịa đặt thông tin…"
- "Nếu liên quan NỘI DUNG tài liệu → GỌI LẠI ragSearch/readDocument ở MỖI lượt (nội dung không giữ giữa các lượt)."
- "Không đủ thông tin → nói rõ chưa có dữ liệu, đừng đoán."

`maxTokens: 4096` để câu trả lời không bị cắt cụt.

---

## 6. Mô hình dữ liệu (MongoDB collections)

| Collection | Nội dung | Ghi bởi |
|---|---|---|
| `conversations` | `{ title, createdAt, updatedAt }` | chat.repository |
| `messages` | `{ conversationId, role, content, toolCalls?, createdAt }` | chat.repository |
| `documents` | chunk bản mới nhất `{ documentId, source, version, chunkIndex, text, embedding, createdAt }` | documents (ingest/update) |
| `document_versions` | bản cũ `{ documentId, version, source, content, archivedAt }` | archiveCurrentVersion |
| `tasks` | `{ title, status, priority, tags, dueDate?, remindAt?, source, createdAt, updatedAt, completedAt? }` | tasks.repository (agent) |

> **Atlas Vector Search Index** `vector_index` trên `documents.embedding` (1024 chiều, cosine) — tạo thủ công trên Atlas UI.

---

## 7. Bảng mapping nhanh: khái niệm → file

| Khái niệm | File |
|---|---|
| Entrypoint / boot | `server.ts`, `app.ts`, `config.ts` |
| Kết nối Mongo (singleton) | `lib/mongo.ts` |
| Embedding (Voyage) | `lib/voyage.ts` |
| Trích text đa định dạng | `modules/documents/extract.ts` |
| Cắt chunk | `modules/documents/chunk.ts` |
| Ingest pipeline | `modules/documents/services/documents.service.ts` |
| Vector search + versioning | `modules/documents/repositories/documents.repository.ts` |
| Endpoint chat (SSE) | `modules/chat/controllers/chat.controller.ts` (+ `services/chat.service.ts`) |
| Lưu/đọc hội thoại | `modules/chat/repositories/chat.repository.ts` |
| Error handler tập trung | `middleware/error-handler.ts` (+ `lib/errors.ts`) |
| **Agent: graph** | `agent/graph.ts` |
| **Agent: 7 tool** | `agent/lc-tools.ts` |
| **Agent: chạy + map SSE** | `agent/graph-runner.ts` |
| Tool task (logic) | `modules/tasks/repositories/tasks.repository.ts` |
| (Legacy) vòng while | `agent/agent-loop.ts` |
| (Legacy) tool Anthropic | `agent/tools.ts` |

---

## 8. Ba khái niệm cốt lõi đọng lại

1. **Tool = năng lực của agent.** Có logic ở backend chưa đủ; phải bọc thành tool (`lc-tools.ts`) thì agent mới dùng được. 7 tool hiện tại = RAG (3) + task (4).
2. **StateGraph = vòng lặp reason→act→observe có cấu trúc.** Node `agent` suy luận, node `tools` hành động, cạnh điều kiện quyết định lặp hay dừng. `state.messages` cộng dồn là bộ nhớ làm việc.
3. **Streaming xuyên suốt.** `streamEvents(v2)` cho token chảy real-time + báo tool đang chạy → SSE → UI hiện chữ dần + chip "đang đọc tài liệu".

> Xem trực quan: `pnpm graph:print` (Mermaid) hoặc bật LangSmith tracing để soi từng node/token/tool/latency của mỗi lượt thật.
