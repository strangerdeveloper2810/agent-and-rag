import type { FastifyInstance } from "fastify";
import {
  createCoversation,
  listConversations,
  getMessages,
  addMessage,
  deleteConversation,
} from "./chat.repository";
import { runAgent, type AgentEvent } from "../../agent/agent-loop";

export const chatRoutes = async (app: FastifyInstance) => {
  app.post("/conversations", async (request) => {
    const body = request.body as { firstMessage?: string };
    return createCoversation(body.firstMessage ?? "");
  });

  app.get("/conversations", async () => listConversations());

  app.get("/conversations/:id/messages", async (req) => {
    const { id } = req.params as { id: string };
    return getMessages(id);
  });

  app.delete("/conversations/:id", async (req) => {
    const { id } = req.params as { id: string };
    return deleteConversation(id);
  });

  // SSE streaming + agent loop: Claude có thể gọi tool (ragSearch, task...) giữa chừng
  app.post("/conversations/:id/chat", async (req, reply) => {
    const { id } = req.params as { id: string };
    const { content } = req.body as { content: string };
    await addMessage(id, "user", content);
    // Chỉ lấy user/assistant cho agent loop (bỏ message role "tool" nếu có)
    const history = (await getMessages(id))
      .filter((m: any) => m.role === "user" || m.role === "assistant")
      .map((m: any) => ({ role: m.role, content: m.content }));

    // Mở kết nối SSE bằng response Node thô (reply.raw)
    reply.raw.writeHead(200, {
      "Content-Type": "text/event-stream",
      "Cache-Control": "no-cache",
      Connection: "keep-alive",
    });

    // Duyệt từng event của agent: text → token; tool_start/tool_end → cho UI hiện badge
    const gen = runAgent(history);
    let full = "";
    let next = await gen.next();
    while (!next.done) {
      const ev: AgentEvent = next.value;
      if (ev.type === "text") {
        full += ev.text;
        reply.raw.write(`data: ${JSON.stringify({ token: ev.text })}\n\n`);
      } else {
        reply.raw.write(`data: ${JSON.stringify(ev)}\n\n`);
      }
      next = await gen.next();
    }
    reply.raw.write(`data: ${JSON.stringify({ done: true })}\n\n`);
    reply.raw.end();

    // Lưu câu trả lời hoàn chỉnh vào DB (bỏ qua nếu rỗng để không làm nhiễu lượt sau)
    if (full.trim().length > 0) {
      await addMessage(id, "assistant", full);
    }
  });
};
