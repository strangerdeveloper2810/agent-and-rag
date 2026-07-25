import type { Collection, ObjectId } from "mongodb";
import { getDb } from "./mongo.module";
import type { MessageRole } from "../../schemas/message";

// ----- Tên collection Mongo — 1 NGUỒN SỰ THẬT -----

export const COLLECTIONS = {
  conversations: "conversations",
  messages: "messages",
  tasks: "tasks",
  documents: "documents",
  documentVersions: "document_versions",
} as const;

// ----- Hình dạng document THỰC SỰ lưu trong Mongo (khác Zod input schema) -----

export interface ConversationDoc {
  _id?: ObjectId;
  tenantId?: string;
  title: string;
  createdAt: Date;
  updatedAt: Date;
}

export interface MessageDoc {
  _id?: ObjectId;
  tenantId?: string;
  conversationId: string;
  role: MessageRole;
  content: string;
  toolCalls?: unknown[];
  createdAt: Date;
}

export interface TaskDoc {
  _id?: ObjectId;
  tenantId?: string;
  title: string;
  status: string;
  priority?: string;
  tags?: string[];
  dueDate?: Date;
  remindAt?: Date;
  source: string;
  createdAt: Date;
  updatedAt: Date;
  completedAt?: Date;
}

/** Chunk tài liệu (bản mới nhất) trong collection `documents`. */
export interface DocChunkDoc {
  _id?: ObjectId;
  tenantId?: string;
  documentId: string;
  source: string;
  version: number;
  chunkIndex: number;
  text: string;
  embedding: number[];
  createdAt: Date;
}

/** Bản đã archive trong `document_versions` (chỉ text, không embedding). */
export interface DocVersionDoc {
  _id?: ObjectId;
  tenantId?: string;
  documentId: string;
  version: number;
  source: string;
  content: string;
  archivedAt: Date;
}

/**
 * Truy cập collection Mongo có TÊN TẬP TRUNG (COLLECTIONS) + KIỂU tường minh.
 * Repository dùng cái này thay `getDb().collection("...")` → hết magic string,
 * và có type-safety trên field khi query/insert.
 */
export const collections = {
  conversations: (): Collection<ConversationDoc> =>
    getDb().collection<ConversationDoc>(COLLECTIONS.conversations),
  messages: (): Collection<MessageDoc> =>
    getDb().collection<MessageDoc>(COLLECTIONS.messages),
  tasks: (): Collection<TaskDoc> =>
    getDb().collection<TaskDoc>(COLLECTIONS.tasks),
  documents: (): Collection<DocChunkDoc> =>
    getDb().collection<DocChunkDoc>(COLLECTIONS.documents),
  documentVersions: (): Collection<DocVersionDoc> =>
    getDb().collection<DocVersionDoc>(COLLECTIONS.documentVersions),
};

/**
 * Tạo index cho các truy vấn thường gặp (idempotent — chạy lại vô hại).
 * Bao gồm cả index cách ly tenant (Phase 3) + index cũ để backward-compatible.
 *
 * LƯU Ý: KHÔNG bao gồm Atlas Vector Search index (`vector_index`) — cái đó phải
 * tạo thủ công trên Atlas UI/API.
 */
export const ensureIndexes = async (): Promise<void> => {
  const database = getDb();

  // Index gốc (backward-compatible, không có tenantId)
  const legacyIndexes = [
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
  ];

  // Index mới cho tenant isolation (Phase 3)
  const tenantIndexes = [
    // conversations: tìm theo tenant, sắp xếp mới nhất
    database
      .collection(COLLECTIONS.conversations)
      .createIndex({ tenantId: 1, updatedAt: -1 }),
    // messages: tìm message của tenant trong 1 conversation
    database
      .collection(COLLECTIONS.messages)
      .createIndex({ tenantId: 1, conversationId: 1, createdAt: 1 }),
    // tasks: lọc theo tenant + status/priority
    database
      .collection(COLLECTIONS.tasks)
      .createIndex({ tenantId: 1, status: 1, priority: 1 }),
    // tasks: tìm theo tenant + tags
    database
      .collection(COLLECTIONS.tasks)
      .createIndex({ tenantId: 1, tags: 1 }),
    // documents: tìm chunk theo tenant + document
    database
      .collection(COLLECTIONS.documents)
      .createIndex({ tenantId: 1, documentId: 1, chunkIndex: 1 }),
    // documents: tìm theo tenant + source
    database
      .collection(COLLECTIONS.documents)
      .createIndex({ tenantId: 1, source: 1 }),
    // document_versions: tìm version theo tenant + document
    database
      .collection(COLLECTIONS.documentVersions)
      .createIndex({ tenantId: 1, documentId: 1, version: 1 }),
  ];

  await Promise.all([...legacyIndexes, ...tenantIndexes]);
}
