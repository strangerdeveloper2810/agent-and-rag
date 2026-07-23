import type { Collection, ObjectId } from "mongodb";
import { getDb, COLLECTIONS } from "./mongo";
import type { MessageRole } from "../schemas/message";

// ----- Hình dạng document THỰC SỰ lưu trong Mongo (khác Zod input schema) -----

export interface ConversationDoc {
  _id?: ObjectId;
  title: string;
  createdAt: Date;
  updatedAt: Date;
}

export interface MessageDoc {
  _id?: ObjectId;
  conversationId: string;
  role: MessageRole;
  content: string;
  toolCalls?: unknown[];
  createdAt: Date;
}

export interface TaskDoc {
  _id?: ObjectId;
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
