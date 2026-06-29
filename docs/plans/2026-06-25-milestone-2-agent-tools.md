# Mốc 2: Agent + Tools (RAG + Task) — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Biến chatbot thành agent: Claude tự quyết định gọi tool (`ragSearch`, `createTask`, `listTasks`, `updateTask`, `deleteTask`). Xây RAG pipeline (upload → chunk → embed → Atlas Vector Search) và agent loop thủ công (vòng `while`).

**Architecture:** Module `documents` lo upload/chunk/embed. Module `tasks` lo CRUD + định nghĩa tool. Một "tool registry" gom tất cả tool. Chat service được nâng cấp: gửi danh sách tool cho Claude, lặp `while` cho tới khi Claude không gọi tool nữa, stream cả token text lẫn event "đang gọi tool" về client.

**Tech Stack:** Fastify, MongoDB Atlas Vector Search, Voyage AI, `@anthropic-ai/sdk` tool use, LangChain `RecursiveCharacterTextSplitter`, Zod, `zod-to-json-schema`.

**Tiền đề:** Mốc 1 đã xong (chat streaming + memory).

---

## Chuẩn bị thủ công: tạo Atlas Vector Search Index

Sau khi Task 2 tạo collection `documents` (chạy upload thử một lần để collection tồn tại), vào **Atlas UI → cluster → Atlas Search → Create Search Index → JSON Editor**, chọn collection `documents`, dán:

```json
{
  "fields": [
    {
      "type": "vector",
      "path": "embedding",
      "numDimensions": 1024,
      "similarity": "cosine"
    }
  ]
}
```
Đặt tên index: `vector_index`. Đợi status "Active".

> **Rủi ro hay gặp:** `numDimensions` phải khớp số chiều Voyage trả về (model `voyage-3` = 1024). Sai số chiều → search lỗi.

---

## Task 1: Voyage embedding client

**Files:**
- Create: `apps/api/src/lib/voyage.ts`
- Test: `apps/api/src/lib/voyage.test.ts`

> Voyage có REST API đơn giản: POST `https://api.voyageai.com/v1/embeddings`.

**Step 1: Write the failing test (test hàm build request body)**
```ts
import { describe, it, expect } from "vitest";
import { buildEmbeddingRequest } from "./voyage.js";

describe("buildEmbeddingRequest", () => {
  it("builds body for documents", () => {
    const body = buildEmbeddingRequest(["a", "b"], "document");
    expect(body.input).toEqual(["a", "b"]);
    expect(body.input_type).toBe("document");
    expect(body.model).toBe("voyage-3");
  });
});
```

**Step 2: Run test → FAIL**
```bash
cd apps/api && pnpm test src/lib/voyage.test.ts
```

**Step 3: Tạo `apps/api/src/lib/voyage.ts`**
```ts
import { config } from "../config.js";

const VOYAGE_URL = "https://api.voyageai.com/v1/embeddings";
const MODEL = "voyage-3";

export function buildEmbeddingRequest(
  texts: string[],
  inputType: "document" | "query",
) {
  return { input: texts, model: MODEL, input_type: inputType };
}

export async function embed(
  texts: string[],
  inputType: "document" | "query",
): Promise<number[][]> {
  const res = await fetch(VOYAGE_URL, {
    method: "POST",
    headers: {
      "content-type": "application/json",
      authorization: `Bearer ${config.VOYAGE_API_KEY}`,
    },
    body: JSON.stringify(buildEmbeddingRequest(texts, inputType)),
  });
  if (!res.ok) throw new Error(`Voyage error: ${res.status} ${await res.text()}`);
  const data = (await res.json()) as { data: { embedding: number[] }[] };
  return data.data.map((d) => d.embedding);
}
```

**Step 4: Run test → PASS**

**Step 5: Commit**
```bash
git add apps/api/src/lib/voyage.ts apps/api/src/lib/voyage.test.ts
git commit -m "feat(api): add voyage embedding client"
```

---

## Task 2: Chunking + Documents repository

**Files:**
- Modify: `apps/api/package.json` (thêm `langchain`, `@langchain/textsplitters`)
- Create: `apps/api/src/modules/documents/chunk.ts`
- Test: `apps/api/src/modules/documents/chunk.test.ts`
- Create: `apps/api/src/modules/documents/documents.repository.ts`

**Step 1: Thêm dependency**
```bash
cd apps/api && pnpm add @langchain/textsplitters
```

**Step 2: Write the failing test**
```ts
import { describe, it, expect } from "vitest";
import { chunkText } from "./chunk.js";

describe("chunkText", () => {
  it("splits long text into multiple chunks", async () => {
    const text = "câu một. ".repeat(500);
    const chunks = await chunkText(text);
    expect(chunks.length).toBeGreaterThan(1);
    expect(chunks[0].length).toBeGreaterThan(0);
  });
  it("returns single chunk for short text", async () => {
    const chunks = await chunkText("ngắn thôi");
    expect(chunks).toEqual(["ngắn thôi"]);
  });
});
```

**Step 3: Run test → FAIL**

**Step 4: Tạo `apps/api/src/modules/documents/chunk.ts`**
```ts
import { RecursiveCharacterTextSplitter } from "@langchain/textsplitters";

const splitter = new RecursiveCharacterTextSplitter({
  chunkSize: 800,
  chunkOverlap: 100,
});

export async function chunkText(text: string): Promise<string[]> {
  return splitter.splitText(text);
}
```

**Step 5: Run test → PASS**

**Step 6: Tạo `apps/api/src/modules/documents/documents.repository.ts`**
```ts
import { getDb } from "../../lib/mongo.js";

export type DocChunk = {
  source: string;
  chunkIndex: number;
  text: string;
  embedding: number[];
  createdAt: Date;
};

export async function insertChunks(chunks: DocChunk[]) {
  if (chunks.length === 0) return;
  await getDb().collection("documents").insertMany(chunks);
}

export async function listSources() {
  return getDb()
    .collection("documents")
    .aggregate([
      { $group: { _id: "$source", chunks: { $sum: 1 } } },
      { $project: { _id: 0, source: "$_id", chunks: 1 } },
    ])
    .toArray();
}

export async function deleteSource(source: string) {
  await getDb().collection("documents").deleteMany({ source });
}

// Vector search dùng Atlas $vectorSearch
export async function searchSimilar(queryEmbedding: number[], k = 5) {
  return getDb()
    .collection("documents")
    .aggregate([
      {
        $vectorSearch: {
          index: "vector_index",
          path: "embedding",
          queryVector: queryEmbedding,
          numCandidates: 100,
          limit: k,
        },
      },
      { $project: { _id: 0, source: 1, text: 1, score: { $meta: "vectorSearchScore" } } },
    ])
    .toArray();
}
```

**Step 7: Commit**
```bash
git add apps/api/src/modules/documents apps/api/package.json
git commit -m "feat(api): add chunking and documents repository with vector search"
```

---

## Task 3: Documents service + upload route

**Files:**
- Modify: `apps/api/package.json` (thêm `@fastify/multipart`)
- Create: `apps/api/src/modules/documents/documents.service.ts`
- Create: `apps/api/src/modules/documents/documents.routes.ts`
- Modify: `apps/api/src/app.ts`

**Step 1: Thêm dependency**
```bash
cd apps/api && pnpm add @fastify/multipart
```

**Step 2: Tạo `apps/api/src/modules/documents/documents.service.ts`**
```ts
import { chunkText } from "./chunk.js";
import { embed } from "../../lib/voyage.js";
import { insertChunks, type DocChunk } from "./documents.repository.js";

export async function ingestDocument(source: string, content: string) {
  const chunks = await chunkText(content);
  const embeddings = await embed(chunks, "document");
  const now = new Date();
  const docs: DocChunk[] = chunks.map((text, i) => ({
    source,
    chunkIndex: i,
    text,
    embedding: embeddings[i],
    createdAt: now,
  }));
  await insertChunks(docs);
  return { source, chunks: docs.length };
}
```

**Step 3: Tạo `apps/api/src/modules/documents/documents.routes.ts`**
```ts
import type { FastifyInstance } from "fastify";
import { ingestDocument } from "./documents.service.js";
import { listSources, deleteSource } from "./documents.repository.js";

export async function documentsRoutes(app: FastifyInstance) {
  app.post("/documents/upload", async (req, reply) => {
    const file = await req.file();
    if (!file) return reply.code(400).send({ error: "Thiếu file" });
    const buffer = await file.toBuffer();
    const content = buffer.toString("utf-8");
    const result = await ingestDocument(file.filename, content);
    return result;
  });

  app.get("/documents", async () => listSources());

  app.delete("/documents/:source", async (req) => {
    const { source } = req.params as { source: string };
    await deleteSource(decodeURIComponent(source));
    return { ok: true };
  });
}
```

**Step 4: Modify `apps/api/src/app.ts` — register multipart + documents routes**
```ts
import multipart from "@fastify/multipart";
import { documentsRoutes } from "./modules/documents/documents.routes.js";
// ...trong buildApp:
app.register(multipart);
app.register(documentsRoutes, { prefix: "/api" });
```

**Step 5: Chạy thử**
```bash
cd apps/api && pnpm dev
# tạo file test.txt rồi:
curl -X POST localhost:3001/api/documents/upload -F file=@test.txt
```
Expected: `{ "source": "test.txt", "chunks": N }`.
→ Sau bước này, **quay lại tạo Atlas Vector Search Index** (phần Chuẩn bị thủ công ở đầu file).

**Step 6: Commit**
```bash
git add apps/api/src/modules/documents apps/api/src/app.ts apps/api/package.json
git commit -m "feat(api): add document upload, ingest pipeline and routes"
```

---

## Task 4: Tasks repository

**Files:**
- Create: `apps/api/src/schemas/task.ts`
- Create: `apps/api/src/modules/tasks/tasks.repository.ts`
- Test: `apps/api/src/schemas/task.test.ts`

**Step 1: Write the failing test**
```ts
import { describe, it, expect } from "vitest";
import { createTaskInputSchema } from "./task.js";

describe("createTaskInputSchema", () => {
  it("applies defaults", () => {
    const t = createTaskInputSchema.parse({ title: "Mua sữa" });
    expect(t.status).toBe("todo");
    expect(t.priority).toBe("medium");
    expect(t.tags).toEqual([]);
  });
  it("rejects empty title", () => {
    expect(() => createTaskInputSchema.parse({ title: "" })).toThrow();
  });
});
```

**Step 2: Run test → FAIL**

**Step 3: Tạo `apps/api/src/schemas/task.ts`**
```ts
import { z } from "zod";

export const taskStatusSchema = z.enum(["todo", "in_progress", "done", "cancelled"]);
export const taskPrioritySchema = z.enum(["low", "medium", "high", "urgent"]);

export const createTaskInputSchema = z.object({
  title: z.string().min(1),
  description: z.string().optional(),
  status: taskStatusSchema.default("todo"),
  priority: taskPrioritySchema.default("medium"),
  tags: z.array(z.string()).default([]),
  dueDate: z.coerce.date().optional(),
  remindAt: z.coerce.date().optional(),
});
export type CreateTaskInput = z.infer<typeof createTaskInputSchema>;

export const updateTaskInputSchema = z.object({
  id: z.string(),
  title: z.string().min(1).optional(),
  description: z.string().optional(),
  status: taskStatusSchema.optional(),
  priority: taskPrioritySchema.optional(),
  tags: z.array(z.string()).optional(),
  dueDate: z.coerce.date().optional(),
  remindAt: z.coerce.date().optional(),
});
export type UpdateTaskInput = z.infer<typeof updateTaskInputSchema>;

export const listTasksInputSchema = z.object({
  status: taskStatusSchema.optional(),
  priority: taskPrioritySchema.optional(),
  tag: z.string().optional(),
});
export type ListTasksInput = z.infer<typeof listTasksInputSchema>;
```

**Step 4: Run test → PASS**

**Step 5: Tạo `apps/api/src/modules/tasks/tasks.repository.ts`**
```ts
import { ObjectId } from "mongodb";
import { getDb } from "../../lib/mongo.js";
import type {
  CreateTaskInput,
  UpdateTaskInput,
  ListTasksInput,
} from "../../schemas/task.js";

function col() {
  return getDb().collection("tasks");
}

export async function createTask(input: CreateTaskInput, source: "user" | "agent") {
  const now = new Date();
  const doc = { ...input, source, createdAt: now, updatedAt: now };
  const res = await col().insertOne(doc);
  return { _id: res.insertedId, ...doc };
}

export async function listTasks(filter: ListTasksInput) {
  const q: Record<string, unknown> = {};
  if (filter.status) q.status = filter.status;
  if (filter.priority) q.priority = filter.priority;
  if (filter.tag) q.tags = filter.tag;
  return col().find(q).sort({ createdAt: -1 }).toArray();
}

export async function updateTask(input: UpdateTaskInput) {
  const { id, ...rest } = input;
  const set: Record<string, unknown> = { ...rest, updatedAt: new Date() };
  if (rest.status === "done") set.completedAt = new Date();
  await col().updateOne({ _id: new ObjectId(id) }, { $set: set });
  return col().findOne({ _id: new ObjectId(id) });
}

export async function deleteTask(id: string) {
  await col().deleteOne({ _id: new ObjectId(id) });
  return { ok: true };
}
```

**Step 6: Commit**
```bash
git add apps/api/src/schemas/task.ts apps/api/src/schemas/task.test.ts apps/api/src/modules/tasks/tasks.repository.ts
git commit -m "feat(api): add task schema and repository"
```

---

## Task 5: Tool registry (định nghĩa tool cho Claude)

**Files:**
- Modify: `apps/api/package.json` (thêm `zod-to-json-schema`)
- Create: `apps/api/src/agent/tools.ts`
- Test: `apps/api/src/agent/tools.test.ts`

> **Khái niệm cốt lõi:** mỗi tool gồm `name`, `description`, `input_schema` (JSON Schema từ Zod), và hàm `execute`. Ta gửi `name + description + input_schema` cho Claude; khi Claude trả về `tool_use`, ta tìm tool theo name và chạy `execute`.

**Step 1: Thêm dependency**
```bash
cd apps/api && pnpm add zod-to-json-schema
```

**Step 2: Write the failing test**
```ts
import { describe, it, expect } from "vitest";
import { toolDefinitions, getTool } from "./tools.js";

describe("tool registry", () => {
  it("exposes anthropic tool definitions", () => {
    const names = toolDefinitions.map((t) => t.name);
    expect(names).toContain("ragSearch");
    expect(names).toContain("createTask");
    expect(toolDefinitions[0].input_schema).toBeDefined();
  });
  it("finds tool by name", () => {
    expect(getTool("createTask")).toBeDefined();
    expect(getTool("khong-ton-tai")).toBeUndefined();
  });
});
```

**Step 3: Run test → FAIL**

**Step 4: Tạo `apps/api/src/agent/tools.ts`**
```ts
import { z } from "zod";
import { zodToJsonSchema } from "zod-to-json-schema";
import {
  createTaskInputSchema,
  updateTaskInputSchema,
  listTasksInputSchema,
} from "../schemas/task.js";
import {
  createTask,
  listTasks,
  updateTask,
  deleteTask,
} from "../modules/tasks/tasks.repository.js";
import { embed } from "../lib/voyage.js";
import { searchSimilar } from "../modules/documents/documents.repository.js";

type Tool = {
  name: string;
  description: string;
  schema: z.ZodTypeAny;
  execute: (input: any) => Promise<unknown>;
};

const ragSearchSchema = z.object({
  query: z.string().describe("Câu truy vấn để tìm trong tài liệu"),
});

const deleteTaskSchema = z.object({ id: z.string() });

const tools: Tool[] = [
  {
    name: "ragSearch",
    description:
      "Tìm kiếm thông tin trong các tài liệu người dùng đã nạp. Dùng khi câu hỏi liên quan đến nội dung tài liệu.",
    schema: ragSearchSchema,
    execute: async ({ query }: z.infer<typeof ragSearchSchema>) => {
      const [vec] = await embed([query], "query");
      const results = await searchSimilar(vec, 5);
      return results;
    },
  },
  {
    name: "createTask",
    description:
      "Tạo một task/công việc mới. Trích xuất title, priority, tags, dueDate, remindAt từ yêu cầu người dùng.",
    schema: createTaskInputSchema,
    execute: async (input) => createTask(createTaskInputSchema.parse(input), "agent"),
  },
  {
    name: "listTasks",
    description: "Liệt kê task, có thể lọc theo status, priority, hoặc tag.",
    schema: listTasksInputSchema,
    execute: async (input) => listTasks(listTasksInputSchema.parse(input)),
  },
  {
    name: "updateTask",
    description: "Cập nhật một task theo id (đổi status, priority, title...).",
    schema: updateTaskInputSchema,
    execute: async (input) => updateTask(updateTaskInputSchema.parse(input)),
  },
  {
    name: "deleteTask",
    description: "Xóa một task theo id.",
    schema: deleteTaskSchema,
    execute: async ({ id }: z.infer<typeof deleteTaskSchema>) => deleteTask(id),
  },
];

// Format gửi cho Anthropic API
export const toolDefinitions = tools.map((t) => ({
  name: t.name,
  description: t.description,
  input_schema: zodToJsonSchema(t.schema, { target: "openApi3" }) as Record<string, unknown>,
}));

export function getTool(name: string): Tool | undefined {
  return tools.find((t) => t.name === name);
}
```

**Step 5: Run test → PASS**

**Step 6: Commit**
```bash
git add apps/api/src/agent/tools.ts apps/api/src/agent/tools.test.ts apps/api/package.json
git commit -m "feat(api): add tool registry for rag and tasks"
```

---

## Task 6: Agent loop thủ công (vòng while)

**Files:**
- Create: `apps/api/src/agent/agent-loop.ts`

> **Đây là phần học quan trọng nhất của Mốc 2.** Vòng lặp: gửi messages + tools cho Claude → nếu Claude trả `tool_use` thì chạy tool, đẩy kết quả vào messages, lặp lại → nếu chỉ có text thì xong.

**Step 1: Tạo `apps/api/src/agent/agent-loop.ts`**
```ts
import Anthropic from "@anthropic-ai/sdk";
import { claude, CLAUDE_MODEL } from "../lib/claude.js";
import { toolDefinitions, getTool } from "./tools.js";

const SYSTEM_PROMPT =
  "Bạn là một trợ lý AI có thể tra cứu tài liệu và quản lý task. " +
  "Khi cần thông tin từ tài liệu, dùng tool ragSearch. " +
  "Khi người dùng muốn tạo/sửa/xem/xóa task, dùng các tool task tương ứng. " +
  "Trả lời ngắn gọn, rõ ràng bằng tiếng Việt. Nếu dùng ragSearch, hãy dẫn nguồn (source).";

export type AgentEvent =
  | { type: "text"; text: string }
  | { type: "tool_start"; name: string }
  | { type: "tool_end"; name: string };

// Stream events (text + thông báo tool) về caller
export async function* runAgent(
  history: { role: "user" | "assistant"; content: string }[],
): AsyncGenerator<AgentEvent, string> {
  const messages: Anthropic.MessageParam[] = history.map((m) => ({
    role: m.role,
    content: m.content,
  }));

  let finalText = "";

  // Lặp tối đa N vòng để tránh loop vô hạn
  for (let step = 0; step < 8; step++) {
    const res = await claude.messages.create({
      model: CLAUDE_MODEL,
      max_tokens: 1024,
      system: SYSTEM_PROMPT,
      tools: toolDefinitions as Anthropic.Tool[],
      messages,
    });

    // Phát text ra ngoài
    for (const block of res.content) {
      if (block.type === "text") {
        finalText += block.text;
        yield { type: "text", text: block.text };
      }
    }

    // Nếu Claude không gọi tool → xong
    if (res.stop_reason !== "tool_use") {
      return finalText;
    }

    // Đẩy phản hồi assistant (gồm tool_use) vào lịch sử
    messages.push({ role: "assistant", content: res.content });

    // Chạy từng tool và gom kết quả
    const toolResults: Anthropic.ToolResultBlockParam[] = [];
    for (const block of res.content) {
      if (block.type !== "tool_use") continue;
      yield { type: "tool_start", name: block.name };
      const tool = getTool(block.name);
      let result: unknown;
      try {
        result = tool
          ? await tool.execute(block.input)
          : { error: `Unknown tool ${block.name}` };
      } catch (err) {
        result = { error: String(err) };
      }
      yield { type: "tool_end", name: block.name };
      toolResults.push({
        type: "tool_result",
        tool_use_id: block.id,
        content: JSON.stringify(result),
      });
    }

    // Gửi kết quả tool lại cho Claude để nó trả lời tiếp
    messages.push({ role: "user", content: toolResults });
  }

  return finalText;
}
```

**Step 2: Commit**
```bash
git add apps/api/src/agent/agent-loop.ts
git commit -m "feat(api): add manual agent loop with tool execution"
```

---

## Task 7: Nối agent loop vào chat endpoint (SSE)

**Files:**
- Modify: `apps/api/src/modules/chat/chat.routes.ts`

**Step 1: Thay endpoint chat dùng `runAgent` thay `streamReply`**
```ts
import { runAgent, type AgentEvent } from "../../agent/agent-loop.js";
// ...
app.post("/conversations/:id/chat", async (req, reply) => {
  const { id } = req.params as { id: string };
  const { content } = req.body as { content: string };
  await addMessage(id, "user", content);
  const history = (await getMessages(id))
    .filter((m: any) => m.role === "user" || m.role === "assistant")
    .map((m: any) => ({ role: m.role, content: m.content }));

  reply.raw.writeHead(200, {
    "Content-Type": "text/event-stream",
    "Cache-Control": "no-cache",
    Connection: "keep-alive",
  });

  const gen = runAgent(history);
  let full = "";
  let next = await gen.next();
  while (!next.done) {
    const ev: AgentEvent = next.value;
    if (ev.type === "text") {
      full += ev.text;
      reply.raw.write(`data: ${JSON.stringify({ token: ev.text })}\n\n`);
    } else {
      // tool_start / tool_end → cho UI hiển thị badge
      reply.raw.write(`data: ${JSON.stringify(ev)}\n\n`);
    }
    next = await gen.next();
  }
  reply.raw.write(`data: ${JSON.stringify({ done: true })}\n\n`);
  reply.raw.end();

  await addMessage(id, "assistant", full);
});
```

**Step 2: Chạy thử**
```bash
cd apps/api && pnpm dev
# thử tạo task:
curl -N -X POST localhost:3001/api/conversations/<ID>/chat \
  -H 'content-type: application/json' \
  -d '{"content":"Tạo task mua sữa ưu tiên cao, tag việc nhà"}'
```
Expected: thấy event `tool_start name=createTask`, rồi text xác nhận. Kiểm tra `curl localhost:3001/api/tasks` (Task 9).

**Step 3: Commit**
```bash
git add apps/api/src/modules/chat/chat.routes.ts
git commit -m "feat(api): wire agent loop into chat SSE endpoint"
```

---

## Task 8: Tasks debug route

**Files:**
- Create: `apps/api/src/modules/tasks/tasks.routes.ts`
- Modify: `apps/api/src/app.ts`

**Step 1: Tạo `apps/api/src/modules/tasks/tasks.routes.ts`**
```ts
import type { FastifyInstance } from "fastify";
import { listTasks } from "./tasks.repository.js";

export async function tasksRoutes(app: FastifyInstance) {
  app.get("/tasks", async () => listTasks({}));
}
```

**Step 2: Modify `apps/api/src/app.ts`**
```ts
import { tasksRoutes } from "./modules/tasks/tasks.routes.js";
// ...
app.register(tasksRoutes, { prefix: "/api" });
```

**Step 3: Commit**
```bash
git add apps/api/src/modules/tasks/tasks.routes.ts apps/api/src/app.ts
git commit -m "feat(api): add tasks debug route"
```

---

## Task 9: Frontend — badge "agent đang gọi tool"

**Files:**
- Modify: `apps/web/src/lib/api.ts`
- Modify: `apps/web/src/components/ChatView.tsx`

**Step 1: Mở rộng `streamChat` để bắt event tool — Modify `apps/web/src/lib/api.ts`**
```ts
export async function streamChat(
  conversationId: string,
  content: string,
  onEvent: (
    e: { token?: string; type?: string; name?: string; done?: boolean },
  ) => void,
): Promise<void> {
  const res = await fetch(`/api/conversations/${conversationId}/chat`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ content }),
  });
  const reader = res.body!.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    const lines = buffer.split("\n\n");
    buffer = lines.pop() ?? "";
    for (const line of lines) {
      if (!line.startsWith("data: ")) continue;
      onEvent(JSON.parse(line.slice(6)));
    }
  }
}
```

**Step 2: Cập nhật `ChatView.tsx` xử lý event tool**
Thêm state `toolStatus` và trong `send()` đổi callback:
```tsx
const [toolStatus, setToolStatus] = useState<string | null>(null);
// ...
await streamChat(convId, content, (e) => {
  if (e.type === "tool_start") {
    const label = e.name === "ragSearch" ? "🔍 đang tìm tài liệu..." : `⚙️ ${e.name}...`;
    setToolStatus(label);
  } else if (e.type === "tool_end") {
    setToolStatus(null);
  } else if (e.token) {
    setMessages((m) => {
      const copy = [...m];
      copy[copy.length - 1] = {
        role: "assistant",
        content: copy[copy.length - 1].content + e.token,
      };
      return copy;
    });
  }
});
setToolStatus(null);
```
Và hiển thị badge phía trên ô input:
```tsx
{toolStatus && (
  <div className="px-4 py-1 text-sm text-blue-600">{toolStatus}</div>
)}
```

**Step 3: Chạy thử cả app**
```bash
pnpm dev
```
Trong chat thử: *"Tạo task gọi điện cho khách hàng ưu tiên gấp"* → thấy badge `⚙️ createTask...` rồi xác nhận. Thử upload tài liệu (cần màn Documents — Task 10) rồi hỏi nội dung tài liệu → thấy `🔍 đang tìm tài liệu...`.

**Step 4: Commit**
```bash
git add apps/web/src
git commit -m "feat(web): show agent tool-call status badges"
```

---

## Task 10: Frontend — màn Documents (upload)

**Files:**
- Create: `apps/web/src/components/DocumentsView.tsx`
- Modify: `apps/web/src/App.tsx` (tab switch đơn giản)
- Modify: `apps/web/src/lib/api.ts` (thêm hàm documents)

**Step 1: Thêm hàm documents vào `api.ts`**
```ts
export async function uploadDocument(file: File) {
  const fd = new FormData();
  fd.append("file", file);
  const r = await fetch("/api/documents/upload", { method: "POST", body: fd });
  return r.json();
}
export async function listDocuments() {
  const r = await fetch("/api/documents");
  return r.json() as Promise<{ source: string; chunks: number }[]>;
}
export async function deleteDocument(source: string) {
  await fetch(`/api/documents/${encodeURIComponent(source)}`, { method: "DELETE" });
}
```

**Step 2: Tạo `apps/web/src/components/DocumentsView.tsx`**
```tsx
import { useEffect, useState } from "react";
import { uploadDocument, listDocuments, deleteDocument } from "../lib/api";

export default function DocumentsView() {
  const [docs, setDocs] = useState<{ source: string; chunks: number }[]>([]);
  const [busy, setBusy] = useState(false);

  const refresh = () => listDocuments().then(setDocs);
  useEffect(() => {
    refresh();
  }, []);

  async function onUpload(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    setBusy(true);
    await uploadDocument(file);
    setBusy(false);
    refresh();
  }

  return (
    <div className="max-w-2xl mx-auto p-6">
      <h2 className="text-xl font-bold mb-4">Tài liệu (RAG)</h2>
      <input type="file" accept=".txt,.md" onChange={onUpload} disabled={busy} />
      {busy && <p className="text-sm text-blue-600 mt-2">Đang xử lý...</p>}
      <ul className="mt-4 space-y-2">
        {docs.map((d) => (
          <li key={d.source} className="flex justify-between border rounded p-2">
            <span>
              {d.source} <span className="text-gray-400">({d.chunks} chunks)</span>
            </span>
            <button
              className="text-red-600 text-sm"
              onClick={async () => {
                await deleteDocument(d.source);
                refresh();
              }}
            >
              Xóa
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}
```

**Step 3: Modify `apps/web/src/App.tsx` — tab đơn giản**
```tsx
import { useState } from "react";
import ChatView from "./components/ChatView";
import DocumentsView from "./components/DocumentsView";

export default function App() {
  const [tab, setTab] = useState<"chat" | "docs">("chat");
  return (
    <div className="h-screen flex flex-col">
      <nav className="flex gap-2 border-b p-2 bg-white">
        <button
          className={`px-3 py-1 rounded ${tab === "chat" ? "bg-blue-100" : ""}`}
          onClick={() => setTab("chat")}
        >
          Chat
        </button>
        <button
          className={`px-3 py-1 rounded ${tab === "docs" ? "bg-blue-100" : ""}`}
          onClick={() => setTab("docs")}
        >
          Tài liệu
        </button>
      </nav>
      <div className="flex-1 overflow-hidden">
        {tab === "chat" ? <ChatView /> : <DocumentsView />}
      </div>
    </div>
  );
}
```

**Step 4: Chạy thử end-to-end**
```bash
pnpm dev
```
Upload một file `.txt` có nội dung → sang tab Chat hỏi về nội dung đó → agent gọi `ragSearch` và trả lời có dẫn nguồn.

**Step 5: Commit**
```bash
git add apps/web/src
git commit -m "feat(web): add documents upload view and tab navigation"
```

---

## Task 11 (bổ sung): Tool liệt kê & đọc tài liệu

> **Vì sao cần:** ban đầu agent chỉ có `ragSearch` (tìm ngữ nghĩa) nên KHÔNG trả lời được "có bao nhiêu tài liệu" hay "đọc nội dung tài liệu X". `listSources` đã có nhưng chỉ là REST API, chưa expose thành tool. Bài học: **agent chỉ làm được những gì ta cấp tool** — có logic ở backend ≠ agent dùng được.
>
> Ta thêm 2 tool: `listDocuments` (liệt kê tên + số chunk) và `readDocument` (đọc toàn bộ nội dung một tài liệu theo tên).

**Files:**
- Modify: `apps/api/src/modules/documents/documents.repository.ts` (thêm `getDocumentContent`)
- Modify: `apps/api/src/agent/tools.ts` (thêm 2 tool)
- Modify: `apps/api/src/agent/agent-loop.ts` (cập nhật system prompt)

**Step 1: Thêm `getDocumentContent` vào `documents.repository.ts`**
```ts
// Đọc toàn bộ 1 tài liệu: gom các chunk theo thứ tự rồi ghép lại
export async function getDocumentContent(source: string) {
  const chunks = await getDb()
    .collection("documents")
    .find({ source })
    .sort({ chunkIndex: 1 })
    .project({ _id: 0, text: 1 })
    .toArray();
  return {
    source,
    found: chunks.length > 0,
    chunks: chunks.length,
    content: chunks.map((c) => c.text).join("\n\n"),
  };
}
```
> Lưu ý: chunk có overlap (~100 ký tự) nên nội dung ghép lại có thể lặp nhẹ ở ranh giới — chấp nhận được cho mục đích đọc. (Muốn chính xác tuyệt đối thì phải lưu thêm text gốc lúc upload.)

**Step 2: Thêm 2 tool vào `tools.ts`**
```ts
// import thêm getDocumentContent (và listSources nếu chưa có)
import {
  searchSimilar,
  listSources,
  getDocumentContent,
} from "../modules/documents/documents.repository";

const listDocumentsSchema = z.object({});
const readDocumentSchema = z.object({
  source: z.string().describe("Tên file tài liệu cần đọc, ví dụ test-rag.txt"),
});

// thêm 2 phần tử này vào mảng tools:
{
  name: "listDocuments",
  description:
    "Liệt kê các tài liệu đã nạp (tên file + số chunk). Dùng khi hỏi 'có bao nhiêu tài liệu' hoặc 'có những tài liệu nào'.",
  schema: listDocumentsSchema,
  execute: async () => listSources(),
},
{
  name: "readDocument",
  description:
    "Đọc TOÀN BỘ nội dung của MỘT tài liệu theo tên file. Dùng khi người dùng muốn xem nội dung đầy đủ của một tài liệu cụ thể. Nếu chưa biết tên file, gọi listDocuments trước.",
  schema: readDocumentSchema,
  execute: async ({ source }: z.infer<typeof readDocumentSchema>) =>
    getDocumentContent(source),
},
```
> Ghi chú: tool `listDocuments` có thể đã được thêm từ trước trong session — nếu vậy chỉ cần thêm `readDocument`.

**Step 3: Cập nhật `SYSTEM_PROMPT` trong `agent-loop.ts`**
Thêm 2 câu để Claude biết khi nào dùng:
```ts
"Khi người dùng hỏi có bao nhiêu/những tài liệu nào, dùng tool listDocuments. " +
"Khi người dùng muốn đọc nội dung đầy đủ một tài liệu, dùng tool readDocument (truyền tên file). " +
```

**Step 4: Chạy thử**
```bash
cd apps/api && pnpm dev
# trong chat hỏi: "Có bao nhiêu tài liệu?"            → agent gọi listDocuments
# rồi: "Đọc giúp tôi nội dung file test-rag.txt"      → agent gọi readDocument
```
Expected: event `tool_start name=listDocuments` / `readDocument`, rồi trả lời đúng nội dung.

**Step 5: Commit**
```bash
git add apps/api/src/agent apps/api/src/modules/documents/documents.repository.ts
git commit -m "feat(api): add listDocuments and readDocument agent tools"
```

> ⚠️ Cân nhắc: `readDocument` trả nguyên văn tài liệu → tài liệu dài sẽ tốn nhiều token. Với dự án học (file nhỏ) thì ổn. Production nên giới hạn độ dài hoặc tóm tắt.

---

## Task 12 (bổ sung, nâng cao): Giữ kết quả tool qua các lượt hội thoại

> **Vấn đề:** hiện chỉ lưu *text trả lời* của assistant vào DB. Các khối `tool_use` (Claude gọi tool) và `tool_result` (kết quả, vd nội dung RAG) **chỉ tồn tại trong bộ nhớ tạm của 1 lượt** rồi mất. Sang lượt sau, agent không còn dữ liệu đã tra → phải gọi lại tool, hoặc dễ **bịa** nếu prompt không chặt.
>
> **Mục tiêu:** lưu đầy đủ "khối hội thoại" để lượt sau Claude vẫn thấy tool_use + tool_result trước đó.

> **Lưu ý ràng buộc của Anthropic API:** mỗi `tool_use` (trong message assistant) phải có `tool_result` tương ứng (`tool_use_id` khớp) nằm ở message `user` NGAY SAU đó. Vì vậy phải lưu & dựng lại đúng cấu trúc khối, không thể chỉ nối text.

**Cách làm (pragmatic — lưu nguyên content blocks):**

**Step 1:** Mở rộng schema message để lưu content dạng khối. Trong `message.ts` thêm field tùy chọn:
```ts
// content có thể là string (như cũ) hoặc mảng block của Anthropic
blocks: z.array(z.any()).optional(), // lưu res.content / toolResults
```

**Step 2:** Sửa agent loop để trả về các message phát sinh trong lượt (không chỉ text).
- `runAgent` thu thập một mảng `turnMessages: Anthropic.MessageParam[]` gồm: `{role:"assistant", content: res.content}` và `{role:"user", content: toolResults}` mỗi vòng có tool.
- Trả về cùng `finalText` (đổi kiểu trả về, hoặc yield thêm 1 event "done" mang turnMessages).

**Step 3:** Ở chat route, lưu các message theo khối:
- Thay vì chỉ `addMessage(id,"assistant", full)`, lưu từng message trong `turnMessages` với `blocks` = content array (role tương ứng). Message user gốc vẫn lưu string như cũ.

**Step 4:** Khi dựng history cho lượt mới (`getMessages` → runAgent):
- Nếu message có `blocks` → dùng `content: m.blocks`; nếu không → `content: m.content` (string).
- Bỏ filter "chỉ user/assistant" cứng nhắc — nhưng vẫn phải giữ ĐÚNG thứ tự và cặp tool_use/tool_result.

**Step 5:** Chạy thử: hỏi về 1 tài liệu (agent gọi ragSearch) → lượt sau hỏi tiếp dựa trên kết quả đó → agent trả lời đúng mà KHÔNG cần gọi ragSearch lại.

> ⚠️ Đánh đổi: context phình to nhanh (mỗi tool_result có thể dài) → tốn token và chạm giới hạn context. Giải pháp thật ở **Mốc 3 (LangGraph)**: quản lý state, cắt tỉa/tóm tắt lịch sử, hoặc chỉ giữ tool_result gần nhất. Task này để *hiểu vấn đề*; bản chính quy làm ở Mốc 3.

**Commit:**
```bash
git commit -m "feat(api): persist tool_use/tool_result across turns"
```

---

## Định nghĩa "Done" cho Mốc 2
- [ ] Upload `.txt/.md` → chunk → embed Voyage → lưu Atlas
- [ ] Atlas Vector Search Index `vector_index` đã Active
- [ ] Hỏi nội dung tài liệu → agent gọi `ragSearch`, trả lời có dẫn nguồn
- [ ] "Tạo task..." → agent gọi `createTask`, task xuất hiện ở `GET /api/tasks`
- [ ] UI hiển thị badge khi agent gọi tool
- [ ] `pnpm test` PASS (voyage, chunk, task schema, tool registry)
- [ ] Hiểu được agent loop thủ công: reason → tool_use → execute → tool_result → lặp
