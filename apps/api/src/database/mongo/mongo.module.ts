import { MongoClient, type Db } from "mongodb";
import { config } from "../../config";

// Re-export từ collections.ts để tạo 1 entry point duy nhất cho tầng Mongo
export { COLLECTIONS, ensureIndexes } from "./collections";
export type {
  ConversationDoc,
  MessageDoc,
  TaskDoc,
  DocChunkDoc,
  DocVersionDoc,
} from "./collections";
export { collections } from "./collections";
export { tenantFilter } from "./tenant.filter";

// ----- Kết nối Mongo -----

let client: MongoClient | null = null;
let db: Db | null = null;

/**
 * Kết nối đến MongoDB. Idempotent — gọi nhiều lần chỉ tạo 1 connection.
 * Dùng db name từ connection string (không cần truyền riêng).
 */
export const connectMongo = async (): Promise<Db> => {
  if (db) return db;
  client = new MongoClient(config.MONGODB_URI);
  await client.connect();
  db = client.db(); // dùng db name trong URI
  return db;
}

/**
 * Lấy instance Db đã kết nối.
 * Throw nếu connectMongo() chưa được gọi.
 */
export const getDb = (): Db => {
  if (!db) throw new Error("Mongo not connected. Call connectMongo() first.");
  return db;
}

/**
 * Đóng kết nối Mongo (graceful shutdown).
 */
export const closeMongo = async (): Promise<void> => {
  await client?.close();
  client = null;
  db = null;
}
