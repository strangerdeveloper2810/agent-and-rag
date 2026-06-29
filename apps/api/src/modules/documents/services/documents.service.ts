import { ObjectId } from "mongodb";
import { chunkText } from "../chunk";
import { embed } from "../../../lib/voyage";
import { extractDocumentText } from "../extract";
import {
  insertChunks,
  archiveCurrentVersion,
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
  const embeddings = await embed(chunks, "document");
  const docs = buildChunkDocs(documentId, source, 1, chunks, embeddings, new Date());
  await insertChunks(docs);
  return { documentId, source, version: 1, chunks: docs.length };
}

/**
 * CẬP NHẬT tài liệu (tạo version mới):
 *  1) archive bản hiện tại sang `document_versions`, xóa khỏi `documents`
 *  2) chunk + embed nội dung mới → lưu với version = cũ + 1
 */
export async function updateDocument(
  documentId: string,
  source: string,
  content: string,
) {
  const prevVersion = await archiveCurrentVersion(documentId);
  if (prevVersion === null) {
    throw new Error(`Tài liệu không tồn tại: ${documentId}`);
  }
  const version = prevVersion + 1;
  const chunks = await chunkText(content);
  const embeddings = await embed(chunks, "document");
  const docs = buildChunkDocs(documentId, source, version, chunks, embeddings, new Date());
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
