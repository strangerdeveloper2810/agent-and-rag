import type { Collection } from "mongodb";
import {
  collections,
  type ConversationDoc,
  type MessageDoc,
  type AttachmentMetaDoc,
} from "../../../lib/collections";
import { toObjectId } from "../../../lib/object-id";
import type { MessageRole } from "../../../schemas/message";
import { isMongoConnected } from "../../../lib/mongo";

// Hàm thuần (không I/O) — giữ standalone để test dễ.
export const buildConversationDocs = (firstMessage: string, now: Date) => {
  const title = firstMessage.trim().slice(0, 50) || "Hội thoại mới";
  return { title, createdAt: now, updatedAt: now };
};

/**
 * Factory tạo chat repository. Nhận getter cho 2 collection (conversations, messages)
 * — mặc định wire vào Mongo thật; test inject fake được. Getter gọi lazy trong method.
 */
export function createChatRepository(
  conversations: () => Collection<ConversationDoc> = collections.conversations,
  messages: () => Collection<MessageDoc> = collections.messages,
) {
  return {
    createConversation: async (firstMessage: string) => {
      const doc = buildConversationDocs(firstMessage, new Date());
      const response = await conversations().insertOne(doc);
      return { _id: response.insertedId, ...doc };
    },

    listConversations: async () => {
      if (!isMongoConnected()) return [];
      return conversations().find().sort({ updatedAt: -1 }).toArray();
    },

    getMessages: async (conversationId: string) => {
      if (!isMongoConnected()) return [];
      return messages()
        .find({ conversationId })
        .sort({ createdAt: 1 })
        .toArray();
    },

    addMessage: async (
      conversationId: string,
      role: MessageRole,
      content: string,
      toolCalls?: unknown[],
      attachments?: AttachmentMetaDoc[],
    ) => {
      const doc: MessageDoc = {
        conversationId,
        role,
        content,
        ...(toolCalls ? { toolCalls } : {}),
        ...(attachments && attachments.length > 0 ? { attachments } : {}),
        createdAt: new Date(),
      };
      await messages().insertOne(doc);
      await conversations().updateOne(
        { _id: toObjectId(conversationId) },
        { $set: { updatedAt: new Date() } },
      );
      return doc;
    },

    /**
     * Nối `additionalContent` vào CUỐI content của assistant message GẦN NHẤT
     * trong hội thoại — dùng cho luồng "Tiếp tục" (continue) khi câu trả lời
     * trước bị cắt vì chạm giới hạn token. Khác `addMessage`: KHÔNG tạo row
     * mới, để lịch sử hiển thị liền mạch 1 message duy nhất kể cả sau khi
     * F5 lại trang (trước đây continue tạo user+assistant message MỚI hoàn
     * toàn, làm code/văn bản dài bị tách đôi vĩnh viễn trong DB).
     *
     * Không tìm thấy assistant message nào (hội thoại trống/lỗi trạng thái)
     * → rơi về tạo mới, không throw (an toàn, vẫn lưu được nội dung thay vì
     * mất trắng).
     */
    appendToLastAssistantMessage: async (
      conversationId: string,
      additionalContent: string,
    ) => {
      const [last] = await messages()
        .find({ conversationId, role: "assistant" })
        .sort({ createdAt: -1 })
        .limit(1)
        .toArray();

      if (!last) {
        const doc: MessageDoc = {
          conversationId,
          role: "assistant",
          content: additionalContent,
          createdAt: new Date(),
        };
        await messages().insertOne(doc);
        await conversations().updateOne(
          { _id: toObjectId(conversationId) },
          { $set: { updatedAt: new Date() } },
        );
        return doc;
      }

      // Update qua aggregation pipeline ($concat) để nối atomic ngay trong
      // Mongo — không đọc content cũ về app rồi ghi lại (tránh race nếu có
      // 2 continue chạy chồng, dù thực tế FE chỉ cho phép 1 request/lúc).
      await messages().updateOne({ _id: last._id }, [
        { $set: { content: { $concat: ["$content", additionalContent] } } },
      ] as never);
      await conversations().updateOne(
        { _id: toObjectId(conversationId) },
        { $set: { updatedAt: new Date() } },
      );

      return { ...last, content: last.content + additionalContent };
    },

    deleteConversation: async (conversationId: string) => {
      const _id = toObjectId(conversationId); // validate sớm, trước khi chạm DB
      await messages().deleteMany({ conversationId });
      await conversations().deleteOne({ _id });
      return { ok: true };
    },
  };
}

// Instance mặc định + named exports (caller không phải đổi).
export const chatRepository = createChatRepository();
export const {
  createConversation,
  listConversations,
  getMessages,
  appendToLastAssistantMessage,
  addMessage,
  deleteConversation,
} = chatRepository;
