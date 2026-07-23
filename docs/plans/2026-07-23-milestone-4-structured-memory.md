# Mốc 4: Structured Memory Hybrid — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Cho agent một bộ nhớ dài hạn thực thụ thay vì mỗi lượt dựng lại từ text history thô. Xây **3 tầng bộ nhớ tường minh**: (1) **Working memory** per-thread bằng LangGraph checkpointer (Mongo), (2) **Episodic/summary** nén hội thoại dài, (3) **Semantic long-term hybrid** = structured store (`memories`) + vector recall (Voyage + Atlas). Kết thúc: agent nhớ được sở thích/sự kiện xuyên hội thoại, có UI xem/sửa memory (human-in-the-loop).

**Architecture:**
```
   MỘT LƯỢT CHAT
   ┌─────────────────────────────────────────────────────────────┐
   │ Tầng 1 — WORKING MEMORY (per-thread)                          │
   │   LangGraph checkpointer (MongoDBSaver), thread_id =          │
   │   conversationId. State đầy đủ (kể cả ToolMessage) → resume.  │
   └───────────────┬─────────────────────────────────────────────┘
                   │ history dài → node summarize
   ┌───────────────▼─────────────────────────────────────────────┐
   │ Tầng 2 — EPISODIC / SUMMARY (rolling)                         │
   │   Nén N message cũ → 1 summary; giữ summary + K gần nhất.     │
   └───────────────┬─────────────────────────────────────────────┘
                   │ trích fact/preference sau turn
   ┌───────────────▼─────────────────────────────────────────────┐
   │ Tầng 3 — SEMANTIC LONG-TERM (HYBRID)                          │
   │   a) Structured: collection `memories`                        │
   │      {type, key, value, confidence, source, embedding?}       │
   │   b) Vector: embed memory → Atlas vector search               │
   │   Recall = merge(structured lookup + vector top-k).           │
   └─────────────────────────────────────────────────────────────┘
```

**Tech Stack:** `@langchain/langgraph-checkpoint-mongodb` (MongoDBSaver), LangGraph `StateGraph` (đã có), Voyage embedding (đã có `embedBatched`), Atlas Vector Search (index thứ 2 `memory_index`), Zod, Vitest.

**Tiền đề:** Mốc 3 đã xong (StateGraph + `graph-runner.runGraph(history, signal)`). Sau các đợt hardening gần đây đã có: `recursionLimit`, AbortSignal xuyên suốt SSE, `readDocument`/`getDocumentContent` theo `documentId`, model factory `agent/model.ts` (chọn provider anthropic|google), Mongo `ensureIndexes()`, error handler tập trung, validate biên bằng Zod.

> **Nguyên tắc:** GIỮ cấu trúc tường minh nhiều tầng (routes→controller→service→repository). Mỗi tính năng memory phải test được ở lớp pure (không I/O) như phần còn lại của repo. Import KHÔNG đuôi `.js` theo convention.

> **Vì sao hybrid, không phải full knowledge graph?** KG đã được HOÃN có chủ đích (xem README roadmap). Hybrid = structured lookup (tất định theo `type/key`) + vector recall (ngữ nghĩa) chạm đủ khái niệm memory của agent thực tế mà nhẹ hơn KG. Có thể nâng lên KG ở Mốc sau nếu cần.

---

## Task 1: Cài checkpointer + wiring thread_id (Working memory)

**Files:**
- Modify: `apps/api/package.json`
- Modify: `apps/api/src/agent/graph.ts`
- Modify: `apps/api/src/agent/graph-runner.ts`
- Modify: `apps/api/src/modules/chat/services/chat.service.ts`

> **Ý tưởng:** Thay vì mỗi lượt truyền lại toàn bộ `history` (đang lọc bỏ `tool` message ở `chat.service.ts`), để LangGraph tự lưu/khôi phục state theo `thread_id = conversationId`. Lượt sau chỉ cần gửi message MỚI; ToolMessage được persist → agent "nhớ" cả kết quả tool trong phiên.

**Step 1: Thêm dependency**
```bash
cd apps/api && pnpm add @langchain/langgraph-checkpoint-mongodb
```

**Step 2: Compile graph với checkpointer** (`agent/graph.ts`)
```ts
import { MongoDBSaver } from "@langchain/langgraph-checkpoint-mongodb";
import { getMongoClient } from "../lib/mongo"; // thêm getter trả về MongoClient

const checkpointer = new MongoDBSaver({ client: getMongoClient() });

export const agentGraph = new StateGraph(MessagesAnnotation)
  .addNode("agent", agentNode)
  .addNode("tools", toolNode)
  .addEdge(START, "agent")
  .addConditionalEdges("agent", shouldContinue, { tools: "tools", [END]: END })
  .addEdge("tools", "agent")
  .compile({ checkpointer });
```
> `lib/mongo.ts`: thêm `export function getMongoClient() { if (!client) throw ...; return client; }` (client đã có sẵn dạng module-level).

**Step 3: `runGraph` nhận `threadId` + chỉ gửi message mới** (`agent/graph-runner.ts`)
```ts
export async function* runGraph(
  input: { role: string; content: string }[], // chỉ message MỚI của lượt này
  opts: { threadId: string; signal?: AbortSignal },
): AsyncGenerator<AgentEvent, string> {
  const stream = agentGraph.streamEvents(
    { messages: toLcMessages(input) },
    {
      version: "v2",
      configurable: { thread_id: opts.threadId },
      signal: opts.signal,
      recursionLimit: 12,
    },
  );
  // ... phần map giữ nguyên
}
```

**Step 4: `chat.service.streamReply` truyền message mới + threadId** — bỏ việc nạp & lọc toàn bộ history (checkpointer lo). Chỉ lấy message user cuối cùng của lượt này.
```ts
export async function* streamReply(conversationId, signal?) {
  const lastUser = await getLastUserMessage(conversationId); // repo helper mới
  let full = "";
  try {
    for await (const ev of runGraph([lastUser], { threadId: conversationId, signal })) {
      if (ev.type === "text") full += ev.text;
      yield ev;
    }
  } finally {
    if (full.trim()) await addMessage(conversationId, "assistant", full);
  }
}
```

**Step 5: Chạy thử resume** — hỏi "tên tôi là Minh", lượt sau hỏi "tôi tên gì?" trong CÙNG hội thoại → agent trả đúng dù không truyền lại history thủ công.

**Step 6: Commit**
```bash
git add -A && git commit -m "feat(api): persist working memory via langgraph mongodb checkpointer"
```

---

## Task 2: Data model + repository `memories` (Semantic store)

**Files:**
- Create: `apps/api/src/schemas/memory.ts`
- Create: `apps/api/src/modules/memory/repositories/memory.repository.ts` (+ `index.ts`)
- Test: `apps/api/src/schemas/memory.test.ts`

**Step 1: Failing test cho schema** (`schemas/memory.test.ts`)
```ts
import { describe, it, expect } from "vitest";
import { memoryInputSchema } from "./memory";

describe("memoryInputSchema", () => {
  it("chấp nhận memory hợp lệ", () => {
    const m = memoryInputSchema.parse({
      type: "preference",
      key: "ngôn_ngữ_trả_lời",
      value: "tiếng Việt",
    });
    expect(m.confidence).toBe(1); // default
  });
  it("từ chối type sai", () => {
    expect(() => memoryInputSchema.parse({ type: "xyz", key: "k", value: "v" })).toThrow();
  });
});
```

**Step 2: Schema** (`schemas/memory.ts`)
```ts
import { z } from "zod";
export const memoryTypeSchema = z.enum(["preference", "fact", "entity"]);
export const memoryInputSchema = z.object({
  type: memoryTypeSchema,
  key: z.string().min(1),
  value: z.string().min(1),
  source: z.enum(["extracted", "user", "agent"]).default("extracted"),
  confidence: z.number().min(0).max(1).default(1),
  conversationId: z.string().optional(),
});
export type MemoryInput = z.infer<typeof memoryInputSchema>;
```

**Step 3: Repository** — CRUD + `upsertMemory` (dedup theo `type+key`) + `searchMemories` ($vectorSearch trên `memories.embedding`, index `memory_index`). Theo đúng khuôn `documents.repository.ts`. Thêm index vào `ensureIndexes()`: `memories { type: 1, key: 1 }`.

**Step 4: Tạo Atlas Vector Search Index thứ 2** `memory_index` trên `memories.embedding` (numDimensions=1024, cosine) — thủ công như `vector_index` (xem plan Mốc 2). Ghi vào README.

**Step 5: Commit**
```bash
git add -A && git commit -m "feat(api): memories schema + hybrid store repository"
```

---

## Task 3: Trích xuất memory sau mỗi lượt (WRITE)

**Files:**
- Create: `apps/api/src/modules/memory/services/extract-memory.ts`
- Test: `apps/api/src/modules/memory/services/extract-memory.test.ts`

> Sau khi lượt chat kết thúc, gọi model (dùng lại `createAgentModel`/model rẻ) trích các `preference/fact/entity` từ đoạn hội thoại → `upsertMemory` (structured + embed). Tách hàm PURE `buildExtractionPrompt(messages)` và `parseExtracted(json)` để test không I/O.

**Step 1: Failing test** cho `parseExtracted` (đầu vào JSON model trả → mảng `MemoryInput` đã validate, bỏ item sai).

**Step 2: Implement** — `extractMemories(conversationId)`:
1. lấy vài message gần nhất,
2. `buildExtractionPrompt` yêu cầu model trả JSON `[{type,key,value,confidence}]`,
3. `parseExtracted` validate qua `memoryInputSchema`,
4. `embedBatched(values, "document")` rồi `upsertMemory`.
> Gọi extract **không chặn** luồng SSE (chạy sau khi `streamReply` lưu assistant xong — fire-and-forget có try/catch, hoặc 1 node cuối graph).

**Step 3: Commit**
```bash
git add -A && git commit -m "feat(api): extract & upsert long-term memories after each turn"
```

---

## Task 4: Hybrid recall trước node agent (READ)

**Files:**
- Modify: `apps/api/src/agent/graph.ts` (thêm node `recall`)
- Create: `apps/api/src/modules/memory/services/recall-memory.ts`
- Test: `apps/api/src/modules/memory/services/recall-memory.test.ts`

> Thêm node `recall` chạy TRƯỚC `agent`: (a) structured lookup theo `type` (vd tất cả `preference`), (b) vector recall top-k theo message user hiện tại, **merge + khử trùng** rồi nhét vào một `SystemMessage` "Bộ nhớ liên quan". Tách hàm PURE `mergeMemories(structured, vector)` (dedup theo `type+key`, ưu tiên confidence cao) để test.

**Step 1: Failing test** cho `mergeMemories` (2 nguồn có item trùng key → giữ 1, chọn confidence cao hơn; giữ thứ tự ổn định).

**Step 2: Implement node `recall`** + sửa graph: `START → recall → agent`, bỏ `START → agent`.
```ts
async function recallNode(state) {
  const memText = await buildMemoryContext(lastUserText(state)); // gọi recall-memory
  return memText
    ? { messages: [new SystemMessage(`Bộ nhớ liên quan về người dùng:\n${memText}`)] }
    : {};
}
```

**Step 3: Chạy thử** — sau khi đã "dạy" agent 1 preference ở hội thoại A, mở hội thoại B MỚI hỏi liên quan → agent áp dụng preference (nhờ recall, không nhờ working memory).

**Step 4: Commit**
```bash
git add -A && git commit -m "feat(api): hybrid memory recall node (structured + vector)"
```

---

## Task 5: Summarization node (Episodic — Tầng 2)

**Files:**
- Modify: `apps/api/src/agent/graph.ts`
- Test: `apps/api/src/agent/summarize.test.ts`

> Conditional edge: nếu `state.messages.length > N` → node `summarize` nén phần cũ thành 1 `SystemMessage` tóm tắt, giữ K message gần nhất. Tránh phình context-window & giảm token. Tách PURE `selectMessagesToSummarize(messages, keepLast)` để test (chọn đúng đoạn cần nén, giữ nguyên K cuối).

**Step 1: Failing test** cho `selectMessagesToSummarize`.

**Step 2: Implement** node + conditional edge trước `agent` (hoặc sau `tools`). Cập nhật `print-graph` để thấy node mới.

**Step 3: Commit**
```bash
git add -A && git commit -m "feat(api): rolling summarization node for long conversations"
```

---

## Task 6: Agentic memory tools (`saveMemory` / `recallMemory`) → 9 tool

**Files:**
- Modify: `apps/api/src/agent/lc-tools.ts` (+ `tools.ts` legacy cho nhất quán)
- Modify: system prompt trong `graph.ts`

> Ngoài memory TỰ ĐỘNG (extract/recall), cho agent CHỦ ĐỘNG ghi/nhớ qua tool — dạy sự khác biệt "automatic vs agentic memory". `saveMemory({type,key,value})` → `upsertMemory`; `recallMemory({query})` → `searchMemories`. Nâng bộ tool 7 → 9.

**Step 1:** thêm 2 tool (Zod schema + execute gọi repository), cập nhật mảng `lcTools` và mô tả tool trong system prompt.

**Step 2: Commit**
```bash
git add -A && git commit -m "feat(api): agentic saveMemory/recallMemory tools"
```

---

## Task 7: UI Memory + human-in-the-loop

**Files:**
- Create: `apps/api/src/modules/memory/{controllers,services}` + `memory.routes.ts` (GET list, DELETE, PATCH)
- Create: `apps/web/src/modules/memory/…` (trang xem/sửa/xoá memory)
- Modify: `apps/web/src/shared/components/Sidebar.tsx` (link "Bộ nhớ")

> Minh bạch: người dùng thấy agent đã nhớ gì, sửa/xoá được fact sai. Đây là **human-in-the-loop** cho memory. Thêm rate-limit/validate như các route khác.

**Step 1:** REST `GET/PATCH/DELETE /api/memories` (layered: controller→service→repository).
**Step 2:** UI list + nút xoá (dùng lại `ConfirmDialog`) + sửa value inline.
**Step 3: Commit**
```bash
git add -A && git commit -m "feat: memory management UI + endpoints (human-in-the-loop)"
```

---

## Task 8: Cập nhật tài liệu

**Files:**
- Modify: `docs/architecture-backend-agent.md` (sơ đồ 3 tầng memory + graph mới)
- Modify: `README.md` (roadmap Mốc 4 → ✅; tính năng memory; env `memory_index`)

**Step 1:** thêm mục "Bộ nhớ 3 tầng" + regenerate Mermaid (`pnpm graph:print`) có node `recall`/`summarize`.

**Step 2: Commit**
```bash
git add -A && git commit -m "docs: milestone 4 structured memory (3-tier hybrid)"
```

---

## Định nghĩa "Done" cho Mốc 4
- [ ] Working memory: cùng một hội thoại nhớ được ngữ cảnh qua nhiều lượt nhờ checkpointer (không truyền history thủ công)
- [ ] Semantic memory: preference/fact "dạy" ở hội thoại A áp dụng được ở hội thoại B MỚI (nhờ recall hybrid)
- [ ] Có node `recall` (structured + vector) và `summarize` trong graph (in được Mermaid mới)
- [ ] Agentic tools `saveMemory`/`recallMemory` hoạt động (9 tool)
- [ ] UI Memory xem/sửa/xoá được (human-in-the-loop)
- [ ] `pnpm test` PASS (schema memory, parseExtracted, mergeMemories, selectMessagesToSummarize) + typecheck + lint + format
- [ ] Hiểu được: working vs episodic vs semantic memory; automatic vs agentic; vì sao hybrid (structured + vector) đủ dùng mà nhẹ hơn KG

---

## Hướng mở rộng sau Mốc 4 (ngoài phạm vi)
- Nâng semantic store thành **knowledge graph** thực thụ (entity + relation) nếu bài toán cần suy luận quan hệ.
- **RAG evaluation harness** (recall@k, faithfulness) để đo memory/RAG có thực sự tốt lên.
- **Guardrails** chống prompt-injection qua nội dung tài liệu/memory; tách read/write tool; xác nhận trước hành động phá hủy.
- Auth/multi-user: gắn `userId` vào memory để mỗi người một bộ nhớ riêng.
