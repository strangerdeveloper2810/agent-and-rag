import { ObjectId } from "mongodb";
import { chunkText } from "../chunk";
import { embedBatched } from "../../../lib/voyage";
import { NotFoundError } from "../../../lib/errors";
import { extractDocumentText } from "../extract";
import {
  insertChunks,
  archiveCurrentVersion,
  getCurrentVersion,
  listDocuments as listDocumentsRepo,
  getVersions as getVersionsRepo,
  getVersionContent as getVersionContentRepo,
  deleteDocument as deleteDocumentRepo,
  type DocChunk,
} from "../repositories";

/**
 * Dựng mảng DocChunk từ chunk + embedding (tách riêng để test thuần, không I/O).
 */
export function buildChunkDocs(
  documentId: string,
  source: string,
  version: number,
  chunks: string[],
  embeddings: number[][],
  now: Date,
): DocChunk[] {
  return chunks.map((text, i) => ({
    documentId,
    source,
    version,
    chunkIndex: i,
    text,
    embedding: embeddings[i],
    createdAt: now,
  }));
}

/**
 * Nạp tài liệu MỚI: sinh documentId mới, version = 1.
 * Pipeline: text thô → chunk → embed (Voyage) → lưu vào `documents`.
 */
export async function ingestDocument(source: string, content: string) {
  const documentId = new ObjectId().toHexString();
  const chunks = await chunkText(content);
  const embeddings = await embedBatched(chunks, "document");
  const docs = buildChunkDocs(
    documentId,
    source,
    1,
    chunks,
    embeddings,
    new Date(),
  );
  await insertChunks(docs);
  return { documentId, source, version: 1, chunks: docs.length };
}

/**
 * CẬP NHẬT tài liệu (tạo version mới).
 *
 * THỨ TỰ QUAN TRỌNG (tránh mất dữ liệu): chunk + embed nội dung mới TRƯỚC, chỉ
 * khi bản mới đã sẵn sàng mới archive bản cũ rồi ghi bản mới. Nếu embed lỗi (vd
 * Voyage 429 — free tier rất dễ dính), bản hiện tại trong `documents` vẫn nguyên
 * vẹn. (Bản cũ đảo thứ tự: xóa trước → embed lỗi → tài liệu biến mất.)
 */
export async function updateDocument(
  documentId: string,
  source: string,
  content: string,
) {
  const current = await getCurrentVersion(documentId);
  if (!current) {
    throw new NotFoundError(`Tài liệu không tồn tại: ${documentId}`);
  }
  const version = current.version + 1;
  const chunks = await chunkText(content);
  const embeddings = await embedBatched(chunks, "document");
  const docs = buildChunkDocs(
    documentId,
    source,
    version,
    chunks,
    embeddings,
    new Date(),
  );
  // Bản mới đã sẵn sàng → giờ mới archive bản cũ và ghi bản mới.
  await archiveCurrentVersion(documentId);
  await insertChunks(docs);
  return { documentId, source, version, chunks: docs.length };
}

// ----- Hàm cho controller gọi (trích text trước rồi mới ingest/update) -----
export async function ingestUpload(filename: string, buffer: Buffer) {
  const content = await extractDocumentText(filename, buffer);
  return ingestDocument(filename, content);
}

export async function updateUpload(
  documentId: string,
  filename: string,
  buffer: Buffer,
) {
  const content = await extractDocumentText(filename, buffer);
  return updateDocument(documentId, filename, content);
}

// ----- Wrapper sang repository (controller chỉ nói chuyện với service) -----
export const listDocuments = () => listDocumentsRepo();
export const getVersions = (documentId: string) => getVersionsRepo(documentId);
export const getVersionContent = (documentId: string, version: number) =>
  getVersionContentRepo(documentId, version);
export const removeDocument = (documentId: string) =>
  deleteDocumentRepo(documentId);
