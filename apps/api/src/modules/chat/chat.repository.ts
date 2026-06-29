import { ObjectId, type Db } from "mongodb";
import { getDb } from "../../lib/mongo";
import type { MessageRole } from "../../schemas/message";

export const buildConversationDocs = (firstMessage: string, now: Date) => {
  const title = firstMessage.trim().slice(0, 50) || "Hội thoại mới";

  return { title, createdAt: now, updatedAt: now };
};

const db = (): Db => getDb();

export const createCoversation = async (firstMessage: string) => {
  const doc = buildConversationDocs(firstMessage, new Date());
  const response = await db().collection("conversations").insertOne(doc);

  return {
    _id: response.insertedId,
    ...doc,
  };
};

export const listConversations = async () =>
  db().collection("conversations").find().sort({ updatedAt: -1 }).toArray();

export const getMessages = async (conversationId: string) =>
  db()
    .collection("messages")
    .find({ conversationId })
    .sort({ createdAt: 1 })
    .toArray();

// Xóa một hội thoại + toàn bộ tin nhắn của nó
export const deleteConversation = async (conversationId: string) => {
  await db().collection("messages").deleteMany({ conversationId });
  await db()
    .collection("conversations")
    .deleteOne({ _id: new ObjectId(conversationId) });
  return { ok: true };
};

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
  await db().collection("messages").insertOne(doc);
  await db()
    .collection("conversations")
    .updateOne(
      { _id: new ObjectId(conversationId) },
      { $set: { updatedAt: new Date() } },
    );
  return doc;
};
