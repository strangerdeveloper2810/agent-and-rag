import type { FastifyReply, FastifyRequest } from "fastify";
import * as chatService from "../services";
import type { AgentEvent } from "../../../agent/graph-runner";

export const postConversation = async (req: FastifyRequest) => {
  const body = req.body as { firstMessage?: string };
  return chatService.createConversation(body.firstMessage ?? "");
};

export const getConversations = async () => chatService.listConversations();

export const getConversationMessages = async (req: FastifyRequest) => {
  const { id } = req.params as { id: string };
  return chatService.getConversationMessages(id);
};

export const deleteConversation = async (req: FastifyRequest) => {
  const { id } = req.params as { id: string };
  return chatService.deleteConversation(id);
};

// Chat qua SSE: agent (LangGraph) có thể gọi tool giữa chừng.
export const postChat = async (req: FastifyRequest, reply: FastifyReply) => {
  const { id } = req.params as { id: string };
  const { content } = req.body as { content: string };

  // Lưu user msg TRƯỚC khi mở SSE → lỗi sớm vẫn trả HTTP bình thường qua error handler
  await chatService.appendUserMessage(id, content);

  reply.raw.writeHead(200, {
    "Content-Type": "text/event-stream",
    "Cache-Control": "no-cache",
    Connection: "keep-alive",
  });

  try {
    for await (const ev of chatService.streamReply(
      id,
    ) as AsyncGenerator<AgentEvent>) {
      if (ev.type === "text") {
        reply.raw.write(`data: ${JSON.stringify({ token: ev.text })}\n\n`);
      } else {
        reply.raw.write(`data: ${JSON.stringify(ev)}\n\n`);
      }
    }
  } catch (err) {
    // Lỗi giữa chừng (đã gửi header) → không thể trả JSON, chỉ log rồi đóng stream
    req.log.error(err);
  }

  reply.raw.write(`data: ${JSON.stringify({ done: true })}\n\n`);
  reply.raw.end();
};
