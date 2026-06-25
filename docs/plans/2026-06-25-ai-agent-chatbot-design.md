# Thiết kế: AI Agent Chatbot (RAG + Task Management)

**Ngày:** 2026-06-25
**Tác giả:** nptdev02
**Mục đích:** Dự án học tập để hiểu cách build AI Agent, sát với techstack của công ty.

---

## 1. Mục tiêu

Xây dựng một AI Agent chatbot vừa học vừa hiểu cách các project AI Agent của công ty vận hành. Agent có 2 năng lực chính:

1. **RAG (Retrieval-Augmented Generation):** tra cứu trong tài liệu người dùng nạp vào, trả lời có dẫn nguồn.
2. **Task management:** tạo / liệt kê / cập nhật / xóa task, lưu vào MongoDB, thông qua function calling.

Đích đến cuối cùng là một **multi-step agent dùng LangGraph**, nhưng được xây **tăng dần qua 3 mốc** để mỗi bước hiểu rõ một khái niệm cốt lõi trước khi nâng độ phức tạp.

### Tiêu chí thành công

- Hiểu và tự giải thích được: agent loop, tool use / function calling, RAG pipeline, vì sao cần LangGraph.
- Chạy được một câu hỏi multi-step kết hợp cả RAG lẫn task (ví dụ: *"Tìm trong tài liệu deadline dự án X rồi tạo task nhắc tôi trước 2 ngày"*).
- Codebase sạch, có test cho phần logic tự viết, sát pattern công ty (Fastify module, Zod, monorepo).

### Ngoài phạm vi (v1)

- Voice STT/TTS (sẽ cắm vào sau — kiến trúc đã chừa chỗ)
- PDF parsing (bắt đầu bằng `.txt`/`.md`)
- Auth / multi-user (single-user)
- Deploy production

---

## 2. Techstack

| Lớp | Công nghệ | Ghi chú |
|-----|-----------|---------|
| Monorepo | pnpm workspaces + Turborepo | quản lý `apps/api` + `apps/web` |
| Backend | Node.js + Fastify + TypeScript | người dùng học Fastify mới |
| Frontend | Vite + React + TypeScript + TailwindCSS | UI tối giản |
| Database | MongoDB Atlas + Atlas Vector Search | một DB cho documents, vectors, tasks, chat |
| LLM | Anthropic Claude (`@anthropic-ai/sdk`) | generate + tool use |
| Embedding | Voyage AI | đối tác embedding Anthropic khuyến nghị |
| Agent orchestration | LangChain + LangGraph | dùng từ Mốc 3 |
| Validation | Zod | schema dùng chung: validate + type + tool input |
| Test | Vitest | unit test logic thuần, mock LLM |
| Streaming | SSE (Server-Sent Events) | stream token trả lời |

**Lưu ý quan trọng:** Anthropic KHÔNG cung cấp API embedding. Claude lo phần generate/reasoning; Voyage AI lo phần text → vector.

---

## 3. Kiến trúc tổng thể

```
┌─────────────────────────────────────────────────────┐
│  apps/web (React + TS + Vite + Tailwind)            │
│  - Màn Chat (gửi tin nhắn, xem trả lời stream)      │
│  - Màn Documents (upload tài liệu cho RAG)          │
└───────────────────────┬─────────────────────────────┘
                        │ HTTP (REST + SSE streaming)
┌───────────────────────▼─────────────────────────────┐
│  apps/api (Node.js + Fastify + TS)                   │
│                                                      │
│   ┌──────────── Agent Core ──────────────────────┐  │
│   │  Claude (LLM)  ←→  Tools:                     │  │
│   │   • ragSearch  • createTask  • listTasks      │  │
│   │   • updateTask • deleteTask                   │  │
│   │  Mốc 2: agent loop thủ công (while)           │  │
│   │  Mốc 3: LangGraph StateGraph                  │  │
│   └───────────────────────────────────────────────┘  │
└──────────┬──────────────────────┬────────────────────┘
           │                      │
   ┌───────▼────────┐   ┌─────────▼──────────┐
   │ MongoDB Atlas  │   │  APIs ngoài        │
   │ - conversations│   │  - Claude (chat)   │
   │ - messages     │   │  - Voyage (embed)  │
   │ - documents    │   │                    │
   │   (+ vectors)  │   │                    │
   │ - tasks        │   │                    │
   └────────────────┘   └────────────────────┘
```

### Triết lý: build tăng dần qua 3 mốc

- **Mốc 1 — Chatbot có memory:** Claude + lưu lịch sử chat vào MongoDB + SSE streaming.
  *Học: gọi LLM, prompt, quản lý hội thoại, streaming.*
- **Mốc 2 — Agent + Tools:** RAG pipeline + task tools qua function calling + agent loop thủ công.
  *Học: tool use, RAG, để LLM tự quyết định gọi tool nào, vòng lặp reason→act→observe viết tay.*
- **Mốc 3 — LangGraph multi-step:** chuyển agent loop thủ công sang StateGraph.
  *Học: agent orchestration thật sự, state machine, multi-step reasoning.*

> Lý do làm tay ở Mốc 2 trước khi dùng LangGraph ở Mốc 3: để **thấy rõ giá trị** LangGraph thay vì coi nó là hộp đen.

---

## 4. Data Model (MongoDB)

Tất cả nằm trong một database trên MongoDB Atlas.

### 4.1 `conversations`
```ts
{
  _id: ObjectId,
  title: string,        // tự sinh từ tin nhắn đầu
  createdAt: Date,
  updatedAt: Date
}
```

### 4.2 `messages`
```ts
{
  _id: ObjectId,
  conversationId: ObjectId,    // ref conversations
  role: "user" | "assistant" | "tool",
  content: string,
  toolCalls?: object[],        // khi assistant gọi tool (Mốc 2+)
  createdAt: Date
}
```

### 4.3 `documents` (chunk + vector cho RAG)
```ts
{
  _id: ObjectId,
  source: string,        // tên file gốc
  chunkIndex: number,    // thứ tự chunk trong file
  text: string,          // nội dung chunk
  embedding: number[],   // vector từ Voyage (1024 chiều)
  createdAt: Date
}
```
→ Cần tạo **Atlas Vector Search Index** trên trường `embedding` (1024 dimensions, similarity: cosine).

### 4.4 `tasks`
```ts
{
  _id: ObjectId,
  title: string,                                       // bắt buộc
  description?: string,
  status: "todo" | "in_progress" | "done" | "cancelled",   // mặc định "todo"
  priority: "low" | "medium" | "high" | "urgent",          // mặc định "medium"
  tags: string[],                                      // vd ["work", "dự án X"]
  dueDate?: Date,
  remindAt?: Date,                                     // thời điểm nhắc
  completedAt?: Date,                                  // set khi status → done
  source: "user" | "agent",                            // ai tạo task này (debug)
  createdAt: Date,
  updatedAt: Date
}
```

**Quyết định YAGNI:** không có collection `users` (single-user), không tách `embeddings` riêng. Vector nằm cùng chunk text trong `documents` (pattern chuẩn của Atlas Vector Search).

**Mọi collection có Zod schema** dùng chung cho: validate input + sinh type TS + làm tool input schema cho Claude.

---

## 5. Agent Core & Tools

### 5.1 Khái niệm Tool

Một tool = hàm TypeScript + mô tả cho Claude. Claude **không tự chạy code** — nó chỉ *quyết định* gọi tool nào với tham số gì; **code của bạn** thực thi và trả kết quả lại cho Claude.

```ts
{
  name: "createTask",
  description: "Tạo một task mới cho người dùng...",  // Claude đọc để biết khi nào gọi
  inputSchema: zodToJsonSchema(CreateTaskSchema),     // tham số
  execute: async (input) => { /* ghi vào MongoDB */ } // code bạn chạy
}
```

### 5.2 Bộ tool

| Tool | Nhiệm vụ | Mốc |
|------|----------|-----|
| `ragSearch` | Embed câu hỏi → vector search trong `documents` → trả chunk liên quan | 2 |
| `createTask` | Tạo task (title, priority, tags, dueDate, remindAt...) | 2 |
| `listTasks` | Liệt kê/lọc task theo status, tag, priority | 2 |
| `updateTask` | Cập nhật status/field của task | 2 |
| `deleteTask` | Xóa task | 2 |

### 5.3 Tiến hóa

- **Mốc 1:** `user → Claude → reply` (không tool).
- **Mốc 2 (agent loop thủ công):**
  ```
  user → Claude → "muốn gọi ragSearch"
       → code chạy ragSearch → đưa kết quả lại Claude
       → Claude → trả lời cuối (hoặc gọi tool tiếp)
  ```
  Vòng `while` lặp đến khi Claude không gọi tool nữa.
- **Mốc 3 (LangGraph):** thay vòng `while` bằng `StateGraph` có node (`agent`, `tools`) + cạnh điều kiện. Quản lý state, dễ thêm node phức tạp, dễ debug/visualize.

---

## 6. RAG Pipeline

### Khi upload tài liệu
```
File .txt/.md
   │ (1) Đọc text thô
   ▼
   │ (2) Chunking — RecursiveCharacterTextSplitter
   │     chunk size ~500-800 token, overlap ~50-100 token
   ▼
[chunk1] [chunk2] [chunk3] ...
   │ (3) Mỗi chunk → Voyage embed → vector
   ▼
[chunk + vector] ...
   │ (4) Lưu vào collection `documents`
   ▼
MongoDB Atlas
```

### Khi user hỏi (qua tool ragSearch)
```
Câu hỏi → Voyage embed → vector câu hỏi
   → Atlas Vector Search: top 3-5 chunk gần nhất (cosine)
   → nhét chunk vào ngữ cảnh prompt Claude
   → Claude trả lời dựa trên chunk (kèm trích dẫn source)
```

**Hai khái niệm chunking:**
- **Chunk size** (~500-800 token): nhỏ quá → mất ngữ cảnh; to quá → nhiễu + tốn tiền.
- **Overlap** (~50-100 token): để câu vắt ngang ranh giới không bị cắt mất nghĩa.

---

## 7. API & Frontend

### 7.1 API Endpoints (Fastify)

**Chat:**
| Method | Route | Nhiệm vụ |
|--------|-------|----------|
| POST | `/api/conversations` | Tạo hội thoại mới |
| GET | `/api/conversations` | Danh sách hội thoại |
| GET | `/api/conversations/:id/messages` | Lịch sử tin nhắn |
| POST | `/api/conversations/:id/chat` | Gửi tin nhắn → stream qua SSE |

**Documents:**
| Method | Route | Nhiệm vụ |
|--------|-------|----------|
| POST | `/api/documents/upload` | Upload .txt/.md → chunk → embed → lưu |
| GET | `/api/documents` | Liệt kê tài liệu đã nạp |
| DELETE | `/api/documents/:source` | Xóa tài liệu |

**Tasks (debug):**
| Method | Route | Nhiệm vụ |
|--------|-------|----------|
| GET | `/api/tasks` | Xem task (kiểm chứng agent tạo đúng) |

> `tasks` được agent tạo **qua tool** trong luồng chat, không phải user gọi REST. `GET /api/tasks` chỉ để quan sát/debug.

### 7.2 Cấu trúc Fastify (theo module)
```
apps/api/src/
  modules/
    chat/        (routes + service + agent loop)
    documents/   (upload, chunk, embed)
    tasks/       (CRUD + tool definitions)
  agent/         (LangGraph graph, tool registry — Mốc 3)
  lib/
    mongo.ts     (kết nối Atlas)
    claude.ts    (client Anthropic)
    voyage.ts    (client embedding)
  schemas/       (Zod schemas dùng chung)
  config.ts      (env validation bằng Zod)
  app.ts         (Fastify app)
  server.ts      (entrypoint)
```

### 7.3 Frontend — 2 màn tối giản

1. **Chat** — danh sách hội thoại bên trái, khung chat bên phải, token chảy dần (SSE), badge nhỏ khi agent gọi tool ("🔍 đang tìm tài liệu...", "✅ đã tạo task").
2. **Documents** — upload file, danh sách tài liệu, nút xóa.

State bằng React hooks thường (chưa cần Redux/Zustand). Đọc SSE bằng `fetch` + `ReadableStream`.

---

## 8. Streaming: SSE (không WebSocket)

Chat AI về bản chất một chiều (server → client stream token), nên SSE đủ và đơn giản hơn WebSocket. Để dành WebSocket cho voice real-time sau này. Claude SDK hỗ trợ streaming sẵn — đọc từng chunk token → đẩy qua SSE về React.

---

## 9. Cấu hình & Testing

### Env (`.env`)
```
ANTHROPIC_API_KEY=...
VOYAGE_API_KEY=...
MONGODB_URI=...
```
→ Validate bằng Zod lúc khởi động (thiếu key → lỗi rõ ràng ngay).

### Testing (Vitest, giữ nhẹ)
- Unit test logic thuần: chunking, parse tool input, Zod schemas, tool execute (ghi DB).
- **Không** test LLM thật → mock client Claude/Voyage.
- Nguyên tắc: test cái *bạn viết*, không test cái bên thứ ba lo.

---

## 10. Rủi ro & lưu ý

- **Atlas Vector Search index phải tạo thủ công** trên trường `embedding` — chỗ người mới hay vướng nhất.
- **Số chiều vector Voyage (1024) phải khớp định nghĩa index** — sai là search lỗi.
- Người dùng **mới với Fastify** → giải thích kỹ concept (routes, plugins, hooks, schema) khi viết code.
- Chi phí API: dùng model rẻ khi học (Claude Haiku), bật chi tiết khi cần.

---

## 11. Lộ trình triển khai (tóm tắt)

- **Mốc 0 — Nền móng:** monorepo (pnpm+Turbo), Fastify hello world, kết nối Atlas, React UI rỗng.
- **Mốc 1 — Chatbot memory:** chat endpoint gọi Claude + lưu messages + SSE streaming.
- **Mốc 2 — Agent + Tools:** RAG pipeline + task tools + agent loop thủ công + badge tool trên UI.
- **Mốc 3 — LangGraph:** chuyển sang StateGraph + multi-step + (tùy chọn) node lập kế hoạch.

Chi tiết bite-sized từng task ở các file plan riêng:
- `2026-06-25-milestone-0-foundation.md`
- `2026-06-25-milestone-1-chatbot-memory.md`
- `2026-06-25-milestone-2-agent-tools.md`
- `2026-06-25-milestone-3-langgraph.md`
