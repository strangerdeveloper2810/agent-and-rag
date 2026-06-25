# Mốc 3: LangGraph multi-step agent — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Thay agent loop thủ công (vòng `while`) bằng một LangGraph `StateGraph` có node và cạnh điều kiện, hỗ trợ multi-step reasoning. Giữ nguyên bộ tool & RAG đã có.

**Architecture:** Định nghĩa state (danh sách messages). Hai node chính: `agent` (gọi Claude qua `ChatAnthropic` của LangChain, đã bind tools) và `tools` (chạy tool). Cạnh điều kiện: nếu Claude gọi tool → đi node `tools` → quay lại `agent`; nếu không → kết thúc. Stream sự kiện graph về SSE như Mốc 2.

**Tech Stack:** `@langchain/langgraph`, `@langchain/anthropic`, `@langchain/core`, Zod, các tool đã có ở Mốc 2.

**Tiền đề:** Mốc 2 đã xong (tool registry, RAG, agent loop thủ công).

> **Vì sao chuyển sang LangGraph?** Vòng `while` ở Mốc 2 hoạt động, nhưng khó mở rộng khi cần: thêm node "lập kế hoạch", "kiểm tra kết quả", rẽ nhánh phức tạp, checkpoint/resume, hay visualize. LangGraph mô hình hóa agent thành đồ thị state machine — đúng cách các project agent thực tế tổ chức.

---

## Task 1: Cài LangGraph + LangChain Anthropic

**Files:**
- Modify: `apps/api/package.json`

**Step 1: Thêm dependencies**
```bash
cd apps/api && pnpm add @langchain/langgraph @langchain/anthropic @langchain/core
```

**Step 2: Commit**
```bash
git add apps/api/package.json apps/api/../../pnpm-lock.yaml
git commit -m "chore(api): add langgraph and langchain anthropic"
```

---

## Task 2: Chuyển tool registry sang LangChain tools

**Files:**
- Create: `apps/api/src/agent/lc-tools.ts`
- Test: `apps/api/src/agent/lc-tools.test.ts`

> LangChain dùng helper `tool()` từ `@langchain/core/tools`: nhận một async function + `{ name, description, schema (Zod) }`. Ta tái dùng đúng logic execute đã viết ở Mốc 2, chỉ bọc lại.

**Step 1: Write the failing test**
```ts
import { describe, it, expect } from "vitest";
import { lcTools } from "./lc-tools.js";

describe("lcTools", () => {
  it("exposes langchain tools with names", () => {
    const names = lcTools.map((t) => t.name);
    expect(names).toContain("ragSearch");
    expect(names).toContain("createTask");
    expect(names).toContain("listTasks");
  });
});
```

**Step 2: Run test → FAIL**

**Step 3: Tạo `apps/api/src/agent/lc-tools.ts`**
```ts
import { tool } from "@langchain/core/tools";
import { z } from "zod";
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

const ragSearch = tool(
  async ({ query }) => {
    const [vec] = await embed([query], "query");
    const results = await searchSimilar(vec, 5);
    return JSON.stringify(results);
  },
  {
    name: "ragSearch",
    description:
      "Tìm kiếm thông tin trong các tài liệu người dùng đã nạp. Dùng khi câu hỏi liên quan đến nội dung tài liệu.",
    schema: z.object({ query: z.string().describe("Câu truy vấn") }),
  },
);

const createTaskTool = tool(
  async (input) => JSON.stringify(await createTask(createTaskInputSchema.parse(input), "agent")),
  {
    name: "createTask",
    description:
      "Tạo một task/công việc mới. Trích xuất title, priority, tags, dueDate, remindAt từ yêu cầu.",
    schema: createTaskInputSchema,
  },
);

const listTasksTool = tool(
  async (input) => JSON.stringify(await listTasks(listTasksInputSchema.parse(input))),
  {
    name: "listTasks",
    description: "Liệt kê task, lọc theo status, priority, hoặc tag.",
    schema: listTasksInputSchema,
  },
);

const updateTaskTool = tool(
  async (input) => JSON.stringify(await updateTask(updateTaskInputSchema.parse(input))),
  {
    name: "updateTask",
    description: "Cập nhật task theo id.",
    schema: updateTaskInputSchema,
  },
);

const deleteTaskTool = tool(
  async ({ id }) => JSON.stringify(await deleteTask(id)),
  {
    name: "deleteTask",
    description: "Xóa task theo id.",
    schema: z.object({ id: z.string() }),
  },
);

export const lcTools = [
  ragSearch,
  createTaskTool,
  listTasksTool,
  updateTaskTool,
  deleteTaskTool,
];
```

**Step 4: Run test → PASS**

**Step 5: Commit**
```bash
git add apps/api/src/agent/lc-tools.ts apps/api/src/agent/lc-tools.test.ts
git commit -m "feat(api): wrap tools as langchain tools"
```

---

## Task 3: Xây StateGraph

**Files:**
- Create: `apps/api/src/agent/graph.ts`

> **Cấu trúc graph:**
> - State = `messages` (cộng dồn).
> - Node `agent`: gọi Claude (đã bind tools) → trả message mới.
> - Node `tools`: `ToolNode` chạy các tool Claude yêu cầu.
> - Cạnh điều kiện sau `agent`: nếu message cuối có `tool_calls` → đi `tools`; ngược lại → `END`.
> - Cạnh `tools → agent` (quay lại để Claude xử lý kết quả).

**Step 1: Tạo `apps/api/src/agent/graph.ts`**
```ts
import { StateGraph, MessagesAnnotation, END, START } from "@langchain/langgraph";
import { ToolNode } from "@langchain/langgraph/prebuilt";
import { ChatAnthropic } from "@langchain/anthropic";
import { SystemMessage } from "@langchain/core/messages";
import { config } from "../config.js";
import { lcTools } from "./lc-tools.js";

const SYSTEM_PROMPT =
  "Bạn là một trợ lý AI có thể tra cứu tài liệu và quản lý task. " +
  "Dùng ragSearch khi cần thông tin từ tài liệu (dẫn nguồn source). " +
  "Dùng các tool task khi người dùng muốn tạo/sửa/xem/xóa task. " +
  "Có thể kết hợp nhiều bước: ví dụ tìm trong tài liệu rồi tạo task. " +
  "Trả lời ngắn gọn, rõ ràng bằng tiếng Việt.";

const model = new ChatAnthropic({
  apiKey: config.ANTHROPIC_API_KEY,
  model: config.CLAUDE_MODEL,
  maxTokens: 1024,
}).bindTools(lcTools);

const toolNode = new ToolNode(lcTools);

async function agentNode(state: typeof MessagesAnnotation.State) {
  const response = await model.invoke([
    new SystemMessage(SYSTEM_PROMPT),
    ...state.messages,
  ]);
  return { messages: [response] };
}

// Quyết định đi tiếp: có tool_calls → "tools", không → END
function shouldContinue(state: typeof MessagesAnnotation.State) {
  const last = state.messages[state.messages.length - 1] as any;
  return last.tool_calls?.length ? "tools" : END;
}

export const agentGraph = new StateGraph(MessagesAnnotation)
  .addNode("agent", agentNode)
  .addNode("tools", toolNode)
  .addEdge(START, "agent")
  .addConditionalEdges("agent", shouldContinue, { tools: "tools", [END]: END })
  .addEdge("tools", "agent")
  .compile();
```

**Step 2: Commit**
```bash
git add apps/api/src/agent/graph.ts
git commit -m "feat(api): build langgraph state graph for agent"
```

---

## Task 4: Stream graph events → SSE

**Files:**
- Create: `apps/api/src/agent/graph-runner.ts`
- Test: `apps/api/src/agent/graph-runner.test.ts`

> LangGraph `streamEvents` (v2) phát event chi tiết: token LLM (`on_chat_model_stream`), tool start/end (`on_tool_start` / `on_tool_end`). Ta map về cùng định dạng `AgentEvent` của Mốc 2 để frontend không phải đổi.

**Step 1: Write the failing test (test hàm map event)**
```ts
import { describe, it, expect } from "vitest";
import { mapGraphEvent } from "./graph-runner.js";

describe("mapGraphEvent", () => {
  it("maps chat model token", () => {
    const out = mapGraphEvent({
      event: "on_chat_model_stream",
      data: { chunk: { content: "xin chào" } },
    });
    expect(out).toEqual({ type: "text", text: "xin chào" });
  });
  it("maps tool start", () => {
    const out = mapGraphEvent({
      event: "on_tool_start",
      name: "ragSearch",
      data: {},
    });
    expect(out).toEqual({ type: "tool_start", name: "ragSearch" });
  });
  it("returns null for irrelevant events", () => {
    expect(mapGraphEvent({ event: "on_chain_start", data: {} })).toBeNull();
  });
});
```

**Step 2: Run test → FAIL**

**Step 3: Tạo `apps/api/src/agent/graph-runner.ts`**
```ts
import { HumanMessage, AIMessage } from "@langchain/core/messages";
import { agentGraph } from "./graph.js";

export type AgentEvent =
  | { type: "text"; text: string }
  | { type: "tool_start"; name: string }
  | { type: "tool_end"; name: string };

// Map một event của LangGraph streamEvents → AgentEvent (hoặc null nếu bỏ qua)
export function mapGraphEvent(ev: any): AgentEvent | null {
  if (ev.event === "on_chat_model_stream") {
    const content = ev.data?.chunk?.content;
    const text =
      typeof content === "string"
        ? content
        : Array.isArray(content)
          ? content.map((c: any) => (typeof c === "string" ? c : c.text ?? "")).join("")
          : "";
    return text ? { type: "text", text } : null;
  }
  if (ev.event === "on_tool_start") return { type: "tool_start", name: ev.name };
  if (ev.event === "on_tool_end") return { type: "tool_end", name: ev.name };
  return null;
}

// Chuyển lịch sử DB → LangChain messages
function toLcMessages(history: { role: string; content: string }[]) {
  return history.map((m) =>
    m.role === "assistant" ? new AIMessage(m.content) : new HumanMessage(m.content),
  );
}

export async function* runGraph(
  history: { role: string; content: string }[],
): AsyncGenerator<AgentEvent, string> {
  const stream = agentGraph.streamEvents(
    { messages: toLcMessages(history) },
    { version: "v2" },
  );

  let finalText = "";
  for await (const ev of stream) {
    const mapped = mapGraphEvent(ev);
    if (!mapped) continue;
    if (mapped.type === "text") finalText += mapped.text;
    yield mapped;
  }
  return finalText;
}
```

**Step 4: Run test → PASS**

**Step 5: Commit**
```bash
git add apps/api/src/agent/graph-runner.ts apps/api/src/agent/graph-runner.test.ts
git commit -m "feat(api): stream langgraph events as agent events"
```

---

## Task 5: Hoán đổi chat endpoint sang graph

**Files:**
- Modify: `apps/api/src/modules/chat/chat.routes.ts`

> Chỉ cần đổi `runAgent` (Mốc 2) → `runGraph`. Định dạng SSE giữ nguyên nên frontend không đổi. Có thể giữ `runAgent` cũ lại để so sánh, nhưng mặc định dùng `runGraph`.

**Step 1: Đổi import và lời gọi trong endpoint chat**
```ts
import { runGraph, type AgentEvent } from "../../agent/graph-runner.js";
// ...
const gen = runGraph(history);
// phần while...next giữ nguyên y như Mốc 2
```

**Step 2: Chạy thử multi-step**
```bash
cd apps/api && pnpm dev
```
Trong UI, thử câu kết hợp (cần đã upload tài liệu có deadline):
*"Tìm trong tài liệu deadline của dự án X rồi tạo task nhắc tôi trước 2 ngày, ưu tiên cao"*
Expected: thấy lần lượt `tool_start ragSearch` → `tool_start createTask` → text xác nhận. Kiểm tra `GET /api/tasks` thấy task mới với `remindAt` đúng.

**Step 3: Commit**
```bash
git add apps/api/src/modules/chat/chat.routes.ts
git commit -m "feat(api): use langgraph in chat endpoint"
```

---

## Task 6: (Tùy chọn) Visualize graph để học

**Files:**
- Create: `apps/api/src/agent/print-graph.ts`

**Step 1: Tạo script in cấu trúc graph dạng Mermaid**
```ts
import { agentGraph } from "./graph.js";

const repr = await agentGraph.getGraphAsync();
console.log(repr.drawMermaid());
```

**Step 2: Chạy**
```bash
cd apps/api && pnpm tsx src/agent/print-graph.ts
```
Expected: in ra sơ đồ Mermaid (START → agent → tools → agent → END). Dán vào https://mermaid.live để xem hình.

**Step 3: Commit**
```bash
git add apps/api/src/agent/print-graph.ts
git commit -m "chore(api): add graph mermaid printer for learning"
```

---

## Task 7: (Tùy chọn) Thêm node "lập kế hoạch"

> Bài tập nâng cao để thấy sức mạnh LangGraph. Thêm node `planner` chạy trước `agent`: yêu cầu Claude phân rã yêu cầu thành các bước, rồi đưa kế hoạch vào context. Đây là pattern thực tế cho câu hỏi phức tạp.

**Files:**
- Modify: `apps/api/src/agent/graph.ts`

**Step 1: Thêm planner node + cạnh `START → planner → agent`**
```ts
async function plannerNode(state: typeof MessagesAnnotation.State) {
  const planModel = new ChatAnthropic({
    apiKey: config.ANTHROPIC_API_KEY,
    model: config.CLAUDE_MODEL,
    maxTokens: 512,
  });
  const userMsg = state.messages[state.messages.length - 1];
  const plan = await planModel.invoke([
    new SystemMessage(
      "Phân rã yêu cầu người dùng thành các bước ngắn gọn (nếu cần dùng tool nào). Chỉ liệt kê bước.",
    ),
    userMsg,
  ]);
  return { messages: [new SystemMessage(`Kế hoạch gợi ý:\n${plan.content}`)] };
}
```
Rồi sửa graph:
```ts
.addNode("planner", plannerNode)
.addEdge(START, "planner")
.addEdge("planner", "agent")
// bỏ .addEdge(START, "agent")
```

**Step 2: Chạy thử & quan sát** câu hỏi phức tạp có thêm bước planner.

**Step 3: Commit**
```bash
git add apps/api/src/agent/graph.ts
git commit -m "feat(api): add optional planner node to graph"
```

---

## Định nghĩa "Done" cho Mốc 3
- [ ] Chat chạy qua LangGraph `StateGraph`, không còn dùng vòng `while` thủ công
- [ ] Streaming token + badge tool vẫn hoạt động (frontend không đổi)
- [ ] Câu hỏi multi-step (RAG + tạo task trong một lượt) chạy đúng
- [ ] `pnpm test` PASS (lc-tools, graph-runner mapping)
- [ ] (Tùy chọn) in được sơ đồ Mermaid của graph
- [ ] Hiểu được: state, node, conditional edge, vì sao LangGraph dễ mở rộng hơn vòng while

---

## Tổng kết toàn dự án
Sau Mốc 3 bạn đã chạm đủ các khái niệm cốt lõi của AI Agent:
- **LLM call + memory** (Mốc 1)
- **Tool use / function calling** + **RAG pipeline** + **agent loop** (Mốc 2)
- **Graph orchestration / state machine / multi-step** (Mốc 3)

**Hướng mở rộng tiếp (ngoài phạm vi):** voice STT/TTS, PDF parsing, auth/multi-user, checkpoint/resume của LangGraph (lưu state vào Mongo), human-in-the-loop (duyệt trước khi agent xóa task), đánh giá (evals) chất lượng RAG.
