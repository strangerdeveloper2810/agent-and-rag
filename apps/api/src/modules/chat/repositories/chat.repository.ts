import type { Collection } from "mongodb";
import {
  collections,
  type ConversationDoc,
  type MessageDoc,
  type AttachmentMetaDoc,
} from "../../../lib/collections";
import { toObjectId } from "../../../lib/object-id";
import type { MessageRole } from "../../../schemas/message";

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

    listConversations: async () =>
      conversations().find().sort({ updatedAt: -1 }).toArray(),

    getMessages: async (conversationId: string) =>
      messages().find({ conversationId }).sort({ createdAt: 1 }).toArray(),

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
  addMessage,
  deleteConversation,
} = chatRepository;
