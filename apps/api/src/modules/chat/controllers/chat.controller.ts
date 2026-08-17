import type { FastifyReply, FastifyRequest } from "fastify";
import * as chatService from "../services";
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

/**
 * Chat qua SSE: stream agent events về client.
 *
 * Flow:
 * 1. Validate + lưu user message vào DB (lỗi sớm, vẫn trả HTTP JSON).
 * 2. Hijack reply, mở SSE stream.
 * 3. Chạy agent, forward từng event -> SSE `data: {...}\n\n`.
 * 4. Khi stream xong -> gửi event `done` kèm metadata (agent name, tokens).
 * 5. Nếu lỗi -> gửi event `error`.
 */
export const postChat = async (req: FastifyRequest, reply: FastifyReply) => {
  const { id } = req.params as { id: string };
  const { content, attachments } = parseOrBadRequest(chatBodySchema, req.body);

  // Lưu user msg TRƯỚC khi mở SSE -> lỗi sớm (validate/DB) vẫn trả HTTP JSON
  // qua error handler (chưa "chiếm" reply nên còn gửi JSON được).
  await chatService.appendUserMessage(id, content, attachments);

  // hijack(): ta tự ghi thẳng vào socket (SSE); Fastify không serialize/gửi nữa.
  reply.hijack();

  // Client đóng kết nối (đóng tab, đổi trang) -> abort agent để không đốt token.
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
    const { events, metadata } = await chatService.streamReply(
      id,
      ac.signal,
      undefined, // use default agent
      attachments,
    );

    for await (const ev of events) {
      if (ev.type === "text") {
        write({ token: ev.text });
      } else if (ev.type === "done") {
        // Go agent gửi event done kèm usage, nhưng ta chỉ dùng để cập nhật
        // metadata bên trong streamReply. Controller tự gửi event done cuối
        // cùng với agent+token info — không forward event done của agent ra SSE.
      } else {
        write(ev);
      }
    }

    // Sau khi stream hoàn tất -> gửi metadata (agent name, tokens used).
    const meta = await metadata;
    write({
      done: true,
      agent: meta.backend,
      tokens: meta.tokensUsed,
      truncated: meta.truncated,
    });
  } catch (err) {
    // Client chủ động ngắt (AbortError) là bình thường — không phải lỗi.
    if (!ac.signal.aborted) {
      req.log.error(err);
      write({ type: "error", message: "Đã xảy ra lỗi khi tạo câu trả lời." });
    }
  }

  if (!reply.raw.writableEnded) reply.raw.end();
};
