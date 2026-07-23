import type { FastifyReply, FastifyRequest } from "fastify";
import * as chatService from "../services";
import type { AgentEvent } from "../../../agent/graph-runner";
import { parseOrBadRequest } from "../../../lib/validate";
import {
  chatBodySchema,
  createConversationBodySchema,
} from "../../../schemas/chat-request";

export const postConversation = async (req: FastifyRequest) => {
  const body = parseOrBadRequest(createConversationBodySchema, req.body);
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
  const { content } = parseOrBadRequest(chatBodySchema, req.body);

  // Lưu user msg TRƯỚC khi mở SSE → lỗi sớm (validate/DB) vẫn trả HTTP bình
  // thường qua error handler (chưa "chiếm" reply nên còn gửi JSON được).
  await chatService.appendUserMessage(id, content);

  // hijack(): ta tự ghi thẳng vào socket (SSE); Fastify không serialize/gửi nữa.
  reply.hijack();

  // Client đóng kết nối (đóng tab, đổi trang) → abort agent để không đốt token.
  const ac = new AbortController();
  req.raw.on("close", () => ac.abort());

  reply.raw.writeHead(200, {
    "Content-Type": "text/event-stream",
    "Cache-Control": "no-cache",
    Connection: "keep-alive",
  });

  const write = (payload: unknown) => {
    if (!reply.raw.writableEnded) {
      reply.raw.write(`data: ${JSON.stringify(payload)}\n\n`);
    }
  };

  try {
    for await (const ev of chatService.streamReply(
      id,
      ac.signal,
    ) as AsyncGenerator<AgentEvent>) {
      if (ev.type === "text") write({ token: ev.text });
      else write(ev);
    }
  } catch (err) {
    // Client chủ động ngắt (AbortError) là bình thường — không phải lỗi.
    if (!ac.signal.aborted) {
      req.log.error(err);
      write({ type: "error", message: "Đã xảy ra lỗi khi tạo câu trả lời." });
    }
  }

  write({ done: true });
  if (!reply.raw.writableEnded) reply.raw.end();
};
