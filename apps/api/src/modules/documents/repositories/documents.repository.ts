import type { Collection } from "mongodb";
import {
  collections,
  type DocChunkDoc,
  type DocVersionDoc,
} from "../../../lib/collections";

// DocChunk = hình dạng chunk tài liệu (định nghĩa tập trung ở lib/collections).
export type DocChunk = DocChunkDoc;

/**
 * Factory tạo document repository. Nhận getter cho 2 collection (documents,
 * document_versions) — mặc định wire vào Mongo thật; test inject fake được.
 * Các hàm nội bộ là local const → gọi lẫn nhau qua closure (vd archive gọi
 * getCurrentVersion).
 */
export function createDocumentRepository(
  docs: () => Collection<DocChunkDoc> = collections.documents,
  versions: () => Collection<DocVersionDoc> = collections.documentVersions,
) {
  const insertChunks = async (chunks: DocChunk[]) => {
    if (chunks.length === 0) return;
    await docs().insertMany(chunks);
  };

  /**
   * Liệt kê tài liệu (bản mới nhất) CỦA MỘT TENANT, gom theo documentId.
   * Mỗi chunk cùng documentId chia sẻ source + version nên $first là an toàn.
   */
  const listDocuments = async (tenantId: string) =>
    docs()
      .aggregate([
        { $match: { tenantId } },
        {
          $group: {
            _id: "$documentId",
            source: { $first: "$source" },
            version: { $first: "$version" },
            chunks: { $sum: 1 },
          },
        },
        {
          $project: {
            _id: 0,
            documentId: "$_id",
            source: 1,
            version: 1,
            chunks: 1,
          },
        },
      ])
      .toArray();

  /**
   * Tìm các chunk gần nghĩa nhất với câu hỏi bằng Atlas $vectorSearch, CHỈ trong
   * phạm vi 1 tenant.
   * - index: "vector_index" (phải khớp tên index tạo trên Atlas)
   * - numCandidates: số ứng viên Atlas quét (nhiều hơn limit để chính xác hơn)
   * - limit (k): số chunk trả về cuối cùng
   * Lọc tenantId bằng $match SAU $vectorSearch (không dùng filter trong-stage):
   * filter trong-stage của $vectorSearch đòi hỏi field phải khai báo type
   * "filter" trong Atlas Search index — index "vector_index" hiện tại chưa có.
   * Không cần lọc version: `documents` đã luôn là bản mới nhất.
   */
  const searchSimilar = async (
    tenantId: string,
    queryEmbedding: number[],
    k = 5,
  ) =>
    docs()
      .aggregate([
        {
          $vectorSearch: {
            index: "vector_index",
            path: "embedding",
            queryVector: queryEmbedding,
            numCandidates: 100,
            limit: k,
          },
        },
        { $match: { tenantId } },
        {
          $project: {
            _id: 0,
            documentId: 1,
            source: 1,
            text: 1,
            score: { $meta: "vectorSearchScore" },
          },
        },
      ])
      .toArray();

  /**
   * Đọc nội dung bản mới nhất theo documentId (tool readDocument dùng), CHỈ nếu
   * thuộc đúng tenant.
   * Dùng documentId (định danh ỔN ĐỊNH, duy nhất) thay vì source (tên file) để
   * tránh trộn nội dung của 2 tài liệu KHÁC NHAU nhưng TRÙNG tên file.
   * Cắt bớt nếu quá dài: tài liệu lớn trả nguyên văn sẽ tốn cả trăm nghìn token.
   */
  const getDocumentContent = async (
    tenantId: string,
    documentId: string,
    maxChars = 24000,
  ) => {
    const chunks = await docs()
      .find({ documentId, tenantId })
      .sort({ chunkIndex: 1 })
      .project({ _id: 0, text: 1, source: 1 })
      .toArray();

    const full = chunks.map((c) => c.text).join("\n\n");
    const truncated = full.length > maxChars;
    return {
      documentId,
      source: chunks[0]?.source as string | undefined,
      found: chunks.length > 0,
      chunks: chunks.length,
      truncated,
      content: truncated ? full.slice(0, maxChars) : full,
      ...(truncated
        ? {
            note: "Tài liệu dài đã bị cắt bớt. Dùng ragSearch để tìm chi tiết cụ thể trong tài liệu.",
          }
        : {}),
    };
  };

  /** Lấy bản hiện tại của 1 tài liệu (ghép text + meta), CHỈ nếu thuộc đúng tenant. null nếu không có. */
  const getCurrentVersion = async (tenantId: string, documentId: string) => {
    const chunks = await docs()
      .find({ documentId, tenantId })
      .sort({ chunkIndex: 1 })
      .toArray();
    if (chunks.length === 0) return null;
    return {
      documentId,
      version: chunks[0].version as number,
      source: chunks[0].source as string,
      content: chunks.map((c) => c.text).join("\n\n"),
    };
  };

  /**
   * Lưu bản hiện tại vào kho lịch sử rồi xóa khỏi `documents`.
   * Trả về số version vừa archive (để caller biết version kế tiếp), null nếu chưa có.
   */
  const archiveCurrentVersion = async (tenantId: string, documentId: string) => {
    const current = await getCurrentVersion(tenantId, documentId);
    if (!current) return null;
    await versions().insertOne({
      tenantId,
      documentId,
      version: current.version,
      source: current.source,
      content: current.content,
      archivedAt: new Date(),
    });
    await docs().deleteMany({ documentId, tenantId });
    return current.version;
  };

  /** Xóa toàn bộ tài liệu (cả bản mới nhất lẫn lịch sử) CỦA MỘT TENANT. */
  const deleteDocument = async (tenantId: string, documentId: string) => {
    await docs().deleteMany({ documentId, tenantId });
    await versions().deleteMany({ documentId, tenantId });
    return { ok: true };
  };

  /** Lịch sử các version (mới → cũ): bản hiện tại + các bản đã archive, CỦA MỘT TENANT. */
  const getVersions = async (tenantId: string, documentId: string) => {
    const current = await getCurrentVersion(tenantId, documentId);
    const archived = await versions()
      .find({ documentId, tenantId })
      .project({ _id: 0, version: 1, source: 1, archivedAt: 1 })
      .toArray();

    const list = [
      ...(current
        ? [{ version: current.version, source: current.source, isLatest: true }]
        : []),
      ...archived.map((a) => ({
        version: a.version as number,
        source: a.source as string,
        isLatest: false,
      })),
    ];
    return list.sort((a, b) => b.version - a.version);
  };

  /** Nội dung một version cụ thể (hiện tại lấy từ `documents`, cũ lấy từ kho), CỦA MỘT TENANT. */
  const getVersionContent = async (
    tenantId: string,
    documentId: string,
    version: number,
  ) => {
    const current = await getCurrentVersion(tenantId, documentId);
    if (current && current.version === version) {
      return {
        found: true,
        documentId,
        version,
        source: current.source,
        content: current.content,
        isLatest: true,
      };
    }
    const archived = await versions().findOne({ documentId, version, tenantId });
    if (!archived) return { found: false };
    return {
      found: true,
      documentId,
      version,
      source: archived.source as string,
      content: archived.content as string,
      isLatest: false,
    };
  };

  return {
    insertChunks,
    listDocuments,
    searchSimilar,
    getDocumentContent,
    getCurrentVersion,
    archiveCurrentVersion,
    deleteDocument,
    getVersions,
    getVersionContent,
  };
}

// Instance mặc định + named exports (caller không phải đổi).
export const documentRepository = createDocumentRepository();
export const {
  insertChunks,
  listDocuments,
  searchSimilar,
  getDocumentContent,
  getCurrentVersion,
  archiveCurrentVersion,
  deleteDocument,
  getVersions,
  getVersionContent,
} = documentRepository;
