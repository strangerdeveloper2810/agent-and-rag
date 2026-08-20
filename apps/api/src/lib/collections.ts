import type { Collection, ObjectId } from "mongodb";
import { getDb, COLLECTIONS } from "./mongo";
import type { MessageRole } from "../schemas/message";

// ----- Hình dạng document THỰC SỰ lưu trong Mongo (khác Zod input schema) -----

export interface ConversationDoc {
  _id?: ObjectId;
  tenantId: string;
  title: string;
  createdAt: Date;
  updatedAt: Date;
}

export interface AttachmentMetaDoc {
  type: "image" | "file";
  name: string;
  size: number;
  mimeType: string;
  thumbnail?: string;
}

export interface MessageDoc {
  _id?: ObjectId;
  tenantId: string;
  conversationId: string;
  role: MessageRole;
  content: string;
  toolCalls?: unknown[];
  attachments?: AttachmentMetaDoc[];
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
  tenantId: string;
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
  tenantId: string;
  documentId: string;
  version: number;
  source: string;
  content: string;
  archivedAt: Date;
}

/** Bản ghi file đã upload trong collection `uploads`. */
export interface UploadDoc {
  _id?: ObjectId;
  tenantId: string;
  userId?: string;
  filename: string;
  originalName: string;
  mimeType: string;
  size: number;
  url: string;
  key: string;
  bucket: string;
  category: string;
  createdAt: Date;
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
  uploads: (): Collection<UploadDoc> =>
    getDb().collection<UploadDoc>(COLLECTIONS.uploads),
};
