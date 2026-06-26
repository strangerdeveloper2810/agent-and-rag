import type { FastifyInstance } from "fastify";
import {
  createCoversation,
  listConversations,
  getMessages,
  addMessage,
} from "./chat.repository";
import { streamReply } from "./chat.service";

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

  // SSE streaming: stream token trả lời về client theo thời gian thực
  app.post("/conversations/:id/chat", async (req, reply) => {
    const { id } = req.params as { id: string };
    const { content } = req.body as { content: string };
    await addMessage(id, "user", content);
    const history = (await getMessages(id)).map((m: any) => ({
      role: m.role,
      content: m.content,
    }));

    // Mở kết nối SSE bằng response Node thô (reply.raw)
    reply.raw.writeHead(200, {
      "Content-Type": "text/event-stream",
      "Cache-Control": "no-cache",
      Connection: "keep-alive",
    });

    // Đọc từng token từ generator → đẩy ngay về client, đồng thời gom lại
    let full = "";
    for await (const token of streamReply(history)) {
      full += token;
      reply.raw.write(`data: ${JSON.stringify({ token })}\n\n`);
    }
    reply.raw.write(`data: ${JSON.stringify({ done: true })}\n\n`);
    reply.raw.end();

    // Lưu câu trả lời hoàn chỉnh vào DB sau khi stream xong
    await addMessage(id, "assistant", full);
  });
};
