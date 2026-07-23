import { MongoClient, type Db } from "mongodb";
import { config } from "../config";

/** Tên collection Mongo — 1 NGUỒN SỰ THẬT (tránh magic string rải rác + typo). */
export const COLLECTIONS = {
  conversations: "conversations",
  messages: "messages",
  tasks: "tasks",
  documents: "documents",
  documentVersions: "document_versions",
} as const;

let client: MongoClient | null = null;
let db: Db | null = null;

export async function connectMongo(): Promise<Db> {
  if (db) return db;
  client = new MongoClient(config.MONGODB_URI);
  await client.connect();
  db = client.db(); // dùng db name trong URI
  return db;
}

export function getDb(): Db {
  if (!db) throw new Error("Mongo not connected. Call connectMongo() first.");
  return db;
}

export async function closeMongo(): Promise<void> {
  await client?.close();
  client = null;
  db = null;
}

/**
 * Tạo index cho các truy vấn thường gặp (idempotent — chạy lại vô hại).
 * LƯU Ý: KHÔNG bao gồm Atlas Vector Search index (`vector_index`) — cái đó phải
 * tạo thủ công trên Atlas UI/API (xem plan Mốc 2).
 */
export async function ensureIndexes(): Promise<void> {
  const database = getDb();
  await Promise.all([
    database
      .collection(COLLECTIONS.messages)
      .createIndex({ conversationId: 1, createdAt: 1 }),
    database
      .collection(COLLECTIONS.conversations)
      .createIndex({ updatedAt: -1 }),
    database
      .collection(COLLECTIONS.tasks)
      .createIndex({ status: 1, priority: 1 }),
    database.collection(COLLECTIONS.tasks).createIndex({ tags: 1 }),
    database
      .collection(COLLECTIONS.documents)
      .createIndex({ documentId: 1, chunkIndex: 1 }),
    database.collection(COLLECTIONS.documents).createIndex({ source: 1 }),
    database
      .collection(COLLECTIONS.documentVersions)
      .createIndex({ documentId: 1, version: 1 }),
  ]);
}
