import { collections } from "../../../lib/collections";
import { toObjectId } from "../../../lib/object-id";
import type { MessageRole } from "../../../schemas/message";

export const buildConversationDocs = (firstMessage: string, now: Date) => {
  const title = firstMessage.trim().slice(0, 50) || "Hội thoại mới";

  return { title, createdAt: now, updatedAt: now };
};

export const createConversation = async (firstMessage: string) => {
  const doc = buildConversationDocs(firstMessage, new Date());
  const response = await collections.conversations().insertOne(doc);

  return {
    _id: response.insertedId,
    ...doc,
  };
};

export const listConversations = async () =>
  collections.conversations().find().sort({ updatedAt: -1 }).toArray();

export const getMessages = async (conversationId: string) =>
  collections
    .messages()
    .find({ conversationId })
    .sort({ createdAt: 1 })
    .toArray();

export const addMessage = async (
  conversationId: string,
  role: MessageRole,
  content: string,
  toolCalls?: unknown[],
) => {
  const doc = {
    conversationId,
    role,
    content,
    ...(toolCalls ? { toolCalls } : {}),
    createdAt: new Date(),
  };
  await collections.messages().insertOne(doc);
  await collections
    .conversations()
    .updateOne(
      { _id: toObjectId(conversationId) },
      { $set: { updatedAt: new Date() } },
    );
  return doc;
};

export const deleteConversation = async (conversationId: string) => {
  const _id = toObjectId(conversationId); // validate sớm, trước khi chạm DB
  await collections.messages().deleteMany({ conversationId });
  await collections.conversations().deleteOne({ _id });
  return { ok: true };
};
