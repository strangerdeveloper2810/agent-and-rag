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
  tenantId: string,
  documentId: string,
  source: string,
  version: number,
  chunks: string[],
  embeddings: number[][],
  now: Date,
): DocChunk[] {
  return chunks.map((text, i) => ({
    tenantId,
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
export async function ingestDocument(
  tenantId: string,
  source: string,
  content: string,
) {
  const documentId = new ObjectId().toHexString();
  const chunks = await chunkText(content, source);
  const embeddings = await embedBatched(chunks, "document");
  const docs = buildChunkDocs(
    tenantId,
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
  tenantId: string,
  documentId: string,
  source: string,
  content: string,
) {
  const current = await getCurrentVersion(tenantId, documentId);
  if (!current) {
    throw new NotFoundError(`Tài liệu không tồn tại: ${documentId}`);
  }
  const version = current.version + 1;
  const chunks = await chunkText(content, source);
  const embeddings = await embedBatched(chunks, "document");
  const docs = buildChunkDocs(
    tenantId,
    documentId,
    source,
    version,
    chunks,
    embeddings,
    new Date(),
  );
  // Bản mới đã sẵn sàng → giờ mới archive bản cũ và ghi bản mới.
  await archiveCurrentVersion(tenantId, documentId);
  await insertChunks(docs);
  return { documentId, source, version, chunks: docs.length };
}

// ----- Hàm cho controller gọi (trích text trước rồi mới ingest/update) -----
export async function ingestUpload(
  tenantId: string,
  filename: string,
  buffer: Buffer,
) {
  const content = await extractDocumentText(filename, buffer);
  return ingestDocument(tenantId, filename, content);
}

// Kết quả nạp cho MỘT file trong lô upload nhiều file (best-effort).
type IngestOk = Awaited<ReturnType<typeof ingestUpload>>;
export type UploadResult =
  | ({ filename: string; ok: true } & IngestOk)
  | { filename: string; ok: false; error: string };

/**
 * Map kết quả Promise.allSettled của MỘT file → UploadResult (tách pure để test).
 */
export function toUploadResult(
  filename: string,
  settled: PromiseSettledResult<IngestOk>,
): UploadResult {
  if (settled.status === "fulfilled") {
    return { filename, ok: true, ...settled.value };
  }
  const reason = settled.reason;
  return {
    filename,
    ok: false,
    error: reason instanceof Error ? reason.message : String(reason),
  };
}

/**
 * Nạp NHIỀU file cùng lúc (song song), best-effort: file lỗi không làm hỏng file
 * khác. Trả về kết quả từng file theo đúng thứ tự đầu vào.
 */
export async function ingestUploads(
  tenantId: string,
  files: { filename: string; buffer: Buffer }[],
): Promise<UploadResult[]> {
  const settled = await Promise.allSettled(
    files.map((f) => ingestUpload(tenantId, f.filename, f.buffer)),
  );
  return files.map((f, i) => toUploadResult(f.filename, settled[i]));
}

export async function updateUpload(
  tenantId: string,
  documentId: string,
  filename: string,
  buffer: Buffer,
) {
  const content = await extractDocumentText(filename, buffer);
  return updateDocument(tenantId, documentId, filename, content);
}

// ----- Wrapper sang repository (controller chỉ nói chuyện với service) -----
export const listDocuments = (tenantId: string) => listDocumentsRepo(tenantId);
export const getVersions = (tenantId: string, documentId: string) =>
  getVersionsRepo(tenantId, documentId);
export const getVersionContent = (
  tenantId: string,
  documentId: string,
  version: number,
) => getVersionContentRepo(tenantId, documentId, version);
export const removeDocument = (tenantId: string, documentId: string) =>
  deleteDocumentRepo(tenantId, documentId);
