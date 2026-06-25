# Mốc 1: Chatbot có memory + SSE streaming — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Chatbot gọi Claude, lưu/đọc lịch sử hội thoại trong MongoDB, và stream câu trả lời về frontend qua SSE.

**Architecture:** Module `chat` trong Fastify quản lý `conversations` + `messages`. Endpoint chat đọc lịch sử từ Mongo, gửi cho Claude (streaming), đẩy token về client qua SSE, rồi lưu tin nhắn assistant. Frontend đọc SSE bằng `fetch` + `ReadableStream`.

**Tech Stack:** Fastify, MongoDB, `@anthropic-ai/sdk` (streaming), Zod, React, SSE.

**Tiền đề:** Mốc 0 đã xong (monorepo, Mongo connected, web gọi được API).

---

## Task 1: Zod schemas cho conversations & messages

**Files:**
- Create: `apps/api/src/schemas/conversation.ts`
- Create: `apps/api/src/schemas/message.ts`
- Test: `apps/api/src/schemas/message.test.ts`

**Step 1: Write the failing test**
```ts
import { describe, it, expect } from "vitest";
import { messageRoleSchema } from "./message.js";

describe("messageRoleSchema", () => {
  it("accepts valid roles", () => {
    expect(messageRoleSchema.parse("user")).toBe("user");
    expect(messageRoleSchema.parse("assistant")).toBe("assistant");
  });
  it("rejects invalid role", () => {
    expect(() => messageRoleSchema.parse("robot")).toThrow();
  });
});
```

**Step 2: Run test to verify it fails**
```bash
cd apps/api && pnpm test src/schemas/message.test.ts
```
Expected: FAIL (`message.js` chưa tồn tại).

**Step 3: Tạo `apps/api/src/schemas/message.ts`**
```ts
import { z } from "zod";

export const messageRoleSchema = z.enum(["user", "assistant", "tool"]);
export type MessageRole = z.infer<typeof messageRoleSchema>;

export const messageSchema = z.object({
  conversationId: z.string(),
  role: messageRoleSchema,
  content: z.string(),
  toolCalls: z.array(z.record(z.unknown())).optional(),
  createdAt: z.date(),
});
export type Message = z.infer<typeof messageSchema>;
```

**Step 4: Tạo `apps/api/src/schemas/conversation.ts`**
```ts
import { z } from "zod";

export const conversationSchema = z.object({
  title: z.string(),
  createdAt: z.date(),
  updatedAt: z.date(),
});
export type Conversation = z.infer<typeof conversationSchema>;
```

**Step 5: Run test to verify it passes**
```bash
cd apps/api && pnpm test src/schemas/message.test.ts
```
Expected: PASS.

**Step 6: Commit**
```bash
git add apps/api/src/schemas
git commit -m "feat(api): add zod schemas for conversation and message"
```

---

## Task 2: Claude client wrapper

**Files:**
- Modify: `apps/api/package.json` (thêm dependency)
- Create: `apps/api/src/lib/claude.ts`

**Step 1: Thêm dependency**
```bash
cd apps/api && pnpm add @anthropic-ai/sdk
```

**Step 2: Tạo `apps/api/src/lib/claude.ts`**
```ts
import Anthropic from "@anthropic-ai/sdk";
import { config } from "../config.js";

export const claude = new Anthropic({ apiKey: config.ANTHROPIC_API_KEY });
export const CLAUDE_MODEL = config.CLAUDE_MODEL;
```

**Step 3: Commit**
```bash
git add apps/api/src/lib/claude.ts apps/api/package.json
git commit -m "feat(api): add anthropic claude client"
```

---

## Task 3: Conversation repository (CRUD Mongo)

**Files:**
- Create: `apps/api/src/modules/chat/chat.repository.ts`
- Test: `apps/api/src/modules/chat/chat.repository.test.ts`

> **Ghi chú test DB:** test này dùng Mongo thật (Atlas) với prefix collection test, hoặc dùng `mongodb-memory-server`. Để đơn giản học, ta test logic mapping bằng cách inject một `Db` giả (mock). Ở đây test hàm tạo document object đúng cấu trúc.

**Step 1: Write the failing test**
```ts
import { describe, it, expect } from "vitest";
import { buildConversationDoc } from "./chat.repository.js";

describe("buildConversationDoc", () => {
  it("creates doc with title and timestamps", () => {
    const now = new Date("2026-06-25T00:00:00Z");
    const doc = buildConversationDoc("Xin chào thế giới này là tiêu đề dài", now);
    expect(doc.title.length).toBeLessThanOrEqual(50);
    expect(doc.createdAt).toEqual(now);
    expect(doc.updatedAt).toEqual(now);
  });
});
```

**Step 2: Run test to verify it fails**
```bash
cd apps/api && pnpm test src/modules/chat/chat.repository.test.ts
```
Expected: FAIL.

**Step 3: Tạo `apps/api/src/modules/chat/chat.repository.ts`**
```ts
import { ObjectId, type Db } from "mongodb";
import { getDb } from "../../lib/mongo.js";
import type { MessageRole } from "../../schemas/message.js";

export function buildConversationDoc(firstMessage: string, now: Date) {
  const title = firstMessage.trim().slice(0, 50) || "Hội thoại mới";
  return { title, createdAt: now, updatedAt: now };
}

function db(): Db {
  return getDb();
}

export async function createConversation(firstMessage: string) {
  const doc = buildConversationDoc(firstMessage, new Date());
  const res = await db().collection("conversations").insertOne(doc);
  return { _id: res.insertedId, ...doc };
}

export async function listConversations() {
  return db()
    .collection("conversations")
    .find()
    .sort({ updatedAt: -1 })
    .toArray();
}

export async function getMessages(conversationId: string) {
  return db()
    .collection("messages")
    .find({ conversationId })
    .sort({ createdAt: 1 })
    .toArray();
}

export async function addMessage(
  conversationId: string,
  role: MessageRole,
  content: string,
  toolCalls?: unknown[],
) {
  const doc = {
    conversationId,
    role,
    content,
    ...(toolCalls ? { toolCalls } : {}),
    createdAt: new Date(),
  };
  await db().collection("messages").insertOne(doc);
  await db()
    .collection("conversations")
    .updateOne({ _id: new ObjectId(conversationId) }, { $set: { updatedAt: new Date() } });
  return doc;
}
```

**Step 4: Run test to verify it passes**
```bash
cd apps/api && pnpm test src/modules/chat/chat.repository.test.ts
```
Expected: PASS.

**Step 5: Commit**
```bash
git add apps/api/src/modules/chat/chat.repository.ts apps/api/src/modules/chat/chat.repository.test.ts
git commit -m "feat(api): add chat repository for conversations and messages"
```

---

## Task 4: Chat service — gọi Claude (non-stream trước)

**Files:**
- Create: `apps/api/src/modules/chat/chat.service.ts`

> Làm bản non-stream trước cho dễ hiểu, Task 6 mới chuyển sang stream.

**Step 1: Tạo `apps/api/src/modules/chat/chat.service.ts`**
```ts
import { claude, CLAUDE_MODEL } from "../../lib/claude.js";
import type { MessageRole } from "../../schemas/message.js";

type ChatMessage = { role: MessageRole; content: string };

const SYSTEM_PROMPT =
  "Bạn là một trợ lý AI hữu ích, trả lời ngắn gọn, rõ ràng bằng tiếng Việt.";

// Chuyển message DB → format Anthropic (chỉ user/assistant)
function toAnthropicMessages(history: ChatMessage[]) {
  return history
    .filter((m) => m.role === "user" || m.role === "assistant")
    .map((m) => ({ role: m.role as "user" | "assistant", content: m.content }));
}

export async function generateReply(history: ChatMessage[]): Promise<string> {
  const res = await claude.messages.create({
    model: CLAUDE_MODEL,
    max_tokens: 1024,
    system: SYSTEM_PROMPT,
    messages: toAnthropicMessages(history),
  });
  const textBlock = res.content.find((b) => b.type === "text");
  return textBlock && textBlock.type === "text" ? textBlock.text : "";
}

export { toAnthropicMessages, SYSTEM_PROMPT };
```

**Step 2: Test cho `toAnthropicMessages` — Create `apps/api/src/modules/chat/chat.service.test.ts`**
```ts
import { describe, it, expect } from "vitest";
import { toAnthropicMessages } from "./chat.service.js";

describe("toAnthropicMessages", () => {
  it("filters out tool messages and maps roles", () => {
    const out = toAnthropicMessages([
      { role: "user", content: "hi" },
      { role: "tool", content: "ignored" },
      { role: "assistant", content: "hello" },
    ]);
    expect(out).toEqual([
      { role: "user", content: "hi" },
      { role: "assistant", content: "hello" },
    ]);
  });
});
```

**Step 3: Run test**
```bash
cd apps/api && pnpm test src/modules/chat/chat.service.test.ts
```
Expected: PASS.

**Step 4: Commit**
```bash
git add apps/api/src/modules/chat/chat.service.ts apps/api/src/modules/chat/chat.service.test.ts
git commit -m "feat(api): add chat service calling claude (non-stream)"
```

---

## Task 5: Chat routes (Fastify plugin)

**Files:**
- Create: `apps/api/src/modules/chat/chat.routes.ts`
- Modify: `apps/api/src/app.ts`

> **Ghi chú Fastify plugin:** mỗi module là một async function nhận `app`. Đăng ký bằng `app.register(plugin, { prefix })`. Đây là cách Fastify chia module gọn gàng.

**Step 1: Tạo `apps/api/src/modules/chat/chat.routes.ts`**
```ts
import type { FastifyInstance } from "fastify";
import {
  createConversation,
  listConversations,
  getMessages,
  addMessage,
} from "./chat.repository.js";
import { generateReply } from "./chat.service.js";

export async function chatRoutes(app: FastifyInstance) {
  app.post("/conversations", async (req) => {
    const body = req.body as { firstMessage?: string };
    return createConversation(body.firstMessage ?? "");
  });

  app.get("/conversations", async () => listConversations());

  app.get("/conversations/:id/messages", async (req) => {
    const { id } = req.params as { id: string };
    return getMessages(id);
  });

  // Bản non-stream tạm thời (Task 6 sẽ thay bằng SSE)
  app.post("/conversations/:id/chat", async (req) => {
    const { id } = req.params as { id: string };
    const { content } = req.body as { content: string };
    await addMessage(id, "user", content);
    const history = (await getMessages(id)).map((m: any) => ({
      role: m.role,
      content: m.content,
    }));
    const reply = await generateReply(history);
    await addMessage(id, "assistant", reply);
    return { reply };
  });
}
```

**Step 2: Modify `apps/api/src/app.ts` — register plugin**
```ts
import Fastify, { type FastifyInstance } from "fastify";
import cors from "@fastify/cors";
import { chatRoutes } from "./modules/chat/chat.routes.js";

export function buildApp(): FastifyInstance {
  const app = Fastify({ logger: true });
  app.register(cors, { origin: true });
  app.get("/api/health", async () => ({ status: "ok" }));
  app.register(chatRoutes, { prefix: "/api" });
  return app;
}
```

**Step 3: Chạy thử bằng curl**
```bash
cd apps/api && pnpm dev
# terminal khác:
curl -X POST localhost:3001/api/conversations -H 'content-type: application/json' -d '{"firstMessage":"Xin chào"}'
# lấy _id rồi:
curl -X POST localhost:3001/api/conversations/<ID>/chat -H 'content-type: application/json' -d '{"content":"Chào bạn"}'
```
Expected: nhận `{ "reply": "..." }` từ Claude.

**Step 4: Commit**
```bash
git add apps/api/src/modules/chat/chat.routes.ts apps/api/src/app.ts
git commit -m "feat(api): add chat routes (non-stream)"
```

---

## Task 6: Chuyển chat sang SSE streaming

**Files:**
- Modify: `apps/api/src/modules/chat/chat.service.ts` (thêm hàm stream)
- Modify: `apps/api/src/modules/chat/chat.routes.ts` (endpoint SSE)

> **Ghi chú SSE:** server set header `Content-Type: text/event-stream`, ghi từng dòng `data: <json>\n\n`. Client đọc từng chunk. Ta dùng `reply.raw` của Fastify để ghi trực tiếp.

**Step 1: Thêm hàm stream vào `chat.service.ts`**
```ts
export async function* streamReply(history: ChatMessage[]): AsyncGenerator<string> {
  const stream = await claude.messages.stream({
    model: CLAUDE_MODEL,
    max_tokens: 1024,
    system: SYSTEM_PROMPT,
    messages: toAnthropicMessages(history),
  });
  for await (const event of stream) {
    if (
      event.type === "content_block_delta" &&
      event.delta.type === "text_delta"
    ) {
      yield event.delta.text;
    }
  }
}
```

**Step 2: Thay endpoint chat trong `chat.routes.ts`**
```ts
app.post("/conversations/:id/chat", async (req, reply) => {
  const { id } = req.params as { id: string };
  const { content } = req.body as { content: string };
  await addMessage(id, "user", content);
  const history = (await getMessages(id)).map((m: any) => ({
    role: m.role,
    content: m.content,
  }));

  reply.raw.writeHead(200, {
    "Content-Type": "text/event-stream",
    "Cache-Control": "no-cache",
    Connection: "keep-alive",
  });

  let full = "";
  for await (const token of streamReply(history)) {
    full += token;
    reply.raw.write(`data: ${JSON.stringify({ token })}\n\n`);
  }
  reply.raw.write(`data: ${JSON.stringify({ done: true })}\n\n`);
  reply.raw.end();

  await addMessage(id, "assistant", full);
});
```
Nhớ import `streamReply` thay `generateReply`.

**Step 3: Chạy thử bằng curl**
```bash
curl -N -X POST localhost:3001/api/conversations/<ID>/chat -H 'content-type: application/json' -d '{"content":"Kể chuyện ngắn"}'
```
Expected: từng dòng `data: {"token":"..."}` chảy ra, kết thúc bằng `data: {"done":true}`.

**Step 4: Commit**
```bash
git add apps/api/src/modules/chat
git commit -m "feat(api): stream chat replies via SSE"
```

---

## Task 7: Frontend — API client đọc SSE

**Files:**
- Create: `apps/web/src/lib/api.ts`

**Step 1: Tạo `apps/web/src/lib/api.ts`**
```ts
export type Conversation = { _id: string; title: string };
export type Message = { _id?: string; role: string; content: string };

export async function createConversation(firstMessage: string): Promise<Conversation> {
  const r = await fetch("/api/conversations", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ firstMessage }),
  });
  return r.json();
}

export async function listConversations(): Promise<Conversation[]> {
  const r = await fetch("/api/conversations");
  return r.json();
}

export async function getMessages(id: string): Promise<Message[]> {
  const r = await fetch(`/api/conversations/${id}/messages`);
  return r.json();
}

// Gửi tin nhắn và stream token về qua callback
export async function streamChat(
  conversationId: string,
  content: string,
  onToken: (token: string) => void,
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
      const data = JSON.parse(line.slice(6));
      if (data.token) onToken(data.token);
    }
  }
}
```

**Step 2: Commit**
```bash
git add apps/web/src/lib/api.ts
git commit -m "feat(web): add api client with SSE chat streaming"
```

---

## Task 8: Frontend — màn Chat

**Files:**
- Create: `apps/web/src/components/ChatView.tsx`
- Modify: `apps/web/src/App.tsx`

**Step 1: Tạo `apps/web/src/components/ChatView.tsx`**
```tsx
import { useEffect, useRef, useState } from "react";
import {
  createConversation,
  listConversations,
  getMessages,
  streamChat,
  type Conversation,
  type Message,
} from "../lib/api";

export default function ChatView() {
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [streaming, setStreaming] = useState(false);
  const endRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    listConversations().then(setConversations);
  }, []);

  useEffect(() => {
    if (activeId) getMessages(activeId).then(setMessages);
  }, [activeId]);

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  async function send() {
    if (!input.trim() || streaming) return;
    const content = input.trim();
    setInput("");

    let convId = activeId;
    if (!convId) {
      const conv = await createConversation(content);
      convId = conv._id;
      setActiveId(convId);
      setConversations((c) => [conv, ...c]);
    }

    setMessages((m) => [...m, { role: "user", content }]);
    setMessages((m) => [...m, { role: "assistant", content: "" }]);
    setStreaming(true);

    await streamChat(convId, content, (token) => {
      setMessages((m) => {
        const copy = [...m];
        copy[copy.length - 1] = {
          role: "assistant",
          content: copy[copy.length - 1].content + token,
        };
        return copy;
      });
    });
    setStreaming(false);
  }

  return (
    <div className="flex h-screen">
      {/* Sidebar hội thoại */}
      <aside className="w-64 border-r bg-gray-50 p-3 overflow-y-auto">
        <button
          className="w-full mb-3 rounded bg-blue-600 text-white py-2 text-sm"
          onClick={() => {
            setActiveId(null);
            setMessages([]);
          }}
        >
          + Hội thoại mới
        </button>
        {conversations.map((c) => (
          <button
            key={c._id}
            onClick={() => setActiveId(c._id)}
            className={`block w-full text-left text-sm p-2 rounded truncate ${
              c._id === activeId ? "bg-blue-100" : "hover:bg-gray-100"
            }`}
          >
            {c.title}
          </button>
        ))}
      </aside>

      {/* Khung chat */}
      <main className="flex-1 flex flex-col">
        <div className="flex-1 overflow-y-auto p-4 space-y-3">
          {messages.map((m, i) => (
            <div
              key={i}
              className={`max-w-2xl rounded-lg p-3 ${
                m.role === "user"
                  ? "ml-auto bg-blue-600 text-white"
                  : "bg-gray-100 text-gray-800"
              }`}
            >
              {m.content || (streaming ? "…" : "")}
            </div>
          ))}
          <div ref={endRef} />
        </div>
        <div className="border-t p-3 flex gap-2">
          <input
            className="flex-1 border rounded px-3 py-2"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && send()}
            placeholder="Nhập tin nhắn..."
            disabled={streaming}
          />
          <button
            onClick={send}
            disabled={streaming}
            className="rounded bg-blue-600 text-white px-4 disabled:opacity-50"
          >
            Gửi
          </button>
        </div>
      </main>
    </div>
  );
}
```

**Step 2: Modify `apps/web/src/App.tsx`**
```tsx
import ChatView from "./components/ChatView";

export default function App() {
  return <ChatView />;
}
```

**Step 3: Chạy thử**
```bash
# từ root
pnpm dev
```
Mở `http://localhost:5173`, gõ tin nhắn → thấy câu trả lời Claude chảy từng chữ.

**Step 4: Commit**
```bash
git add apps/web/src
git commit -m "feat(web): add chat view with streaming UI"
```

---

## Định nghĩa "Done" cho Mốc 1
- [ ] Tạo được hội thoại mới, gửi tin nhắn, nhận trả lời stream từng chữ
- [ ] Lịch sử hội thoại lưu trong MongoDB và load lại khi chọn hội thoại cũ
- [ ] Sidebar hiển thị danh sách hội thoại
- [ ] `pnpm test` PASS (schemas, repository helper, service helper)
- [ ] Hiểu được: agent loop chưa có tool, chỉ là `user → Claude → reply` + memory
