import { getDb } from "../../../lib/mongo";

/**
 * Một chunk của tài liệu (bản MỚI NHẤT) — nằm trong collection `documents`.
 * - documentId: id logic ỔN ĐỊNH của tài liệu, KHÔNG đổi qua các version.
 * - source: tên file của version hiện tại (để hiển thị, có thể đổi khi cập nhật).
 * - version: số phiên bản (1, 2, 3...).
 *
 * `documents` LUÔN chỉ chứa bản mới nhất → searchSimilar/Atlas index không phải đổi.
 * Bản cũ được đẩy sang collection `document_versions` (chỉ text, không embedding).
 */
export type DocChunk = {
  documentId: string;
  source: string;
  version: number;
  chunkIndex: number;
  text: string;
  embedding: number[];
  createdAt: Date;
};

const docs = () => getDb().collection("documents");
const versions = () => getDb().collection("document_versions");

export async function insertChunks(chunks: DocChunk[]) {
  if (chunks.length === 0) return;
  await docs().insertMany(chunks);
}

/**
 * Liệt kê tài liệu (bản mới nhất), gom theo documentId.
 * Mỗi chunk cùng documentId chia sẻ source + version nên $first là an toàn.
 */
export async function listDocuments() {
  return docs()
    .aggregate([
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
}

/**
 * Tìm các chunk gần nghĩa nhất với câu hỏi bằng Atlas $vectorSearch.
 * - index: "vector_index" (phải khớp tên index tạo trên Atlas)
 * - numCandidates: số ứng viên Atlas quét (nhiều hơn limit để chính xác hơn)
 * - limit (k): số chunk trả về cuối cùng
 *
 * Không cần lọc version: `documents` đã luôn là bản mới nhất.
 */
export async function searchSimilar(queryEmbedding: number[], k = 5) {
  return docs()
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
      {
        $project: {
          _id: 0,
          source: 1,
          text: 1,
          score: { $meta: "vectorSearchScore" },
        },
      },
    ])
    .toArray();
}

/**
 * Đọc nội dung bản mới nhất theo TÊN FILE (tool readDocument dùng).
 * Cắt bớt nếu quá dài: tài liệu lớn (vd PDF mấy trăm trang) trả nguyên văn sẽ
 * tốn cả trăm nghìn token → chậm & đắt. Khi bị cắt, báo agent dùng ragSearch.
 */
export async function getDocumentContent(source: string, maxChars = 24000) {
  const chunks = await docs()
    .find({ source })
    .sort({ chunkIndex: 1 })
    .project({ _id: 0, text: 1 })
    .toArray();

  const full = chunks.map((c) => c.text).join("\n\n");
  const truncated = full.length > maxChars;
  return {
    source,
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
}

/** Lấy bản hiện tại của 1 tài liệu (ghép text + meta). null nếu không có. */
export async function getCurrentVersion(documentId: string) {
  const chunks = await docs()
    .find({ documentId })
    .sort({ chunkIndex: 1 })
    .toArray();
  if (chunks.length === 0) return null;
  return {
    documentId,
    version: chunks[0].version as number,
    source: chunks[0].source as string,
    content: chunks.map((c) => c.text).join("\n\n"),
  };
}

/**
 * Lưu bản hiện tại vào kho lịch sử rồi xóa khỏi `documents`.
 * Trả về số version vừa archive (để caller biết version kế tiếp), null nếu chưa có.
 */
export async function archiveCurrentVersion(documentId: string) {
  const current = await getCurrentVersion(documentId);
  if (!current) return null;
  await versions().insertOne({
    documentId,
    version: current.version,
    source: current.source,
    content: current.content,
    archivedAt: new Date(),
  });
  await docs().deleteMany({ documentId });
  return current.version;
}

/** Xóa toàn bộ tài liệu (cả bản mới nhất lẫn lịch sử). */
export async function deleteDocument(documentId: string) {
  await docs().deleteMany({ documentId });
  await versions().deleteMany({ documentId });
  return { ok: true };
}

/** Lịch sử các version (mới → cũ): bản hiện tại + các bản đã archive. */
export async function getVersions(documentId: string) {
  const current = await getCurrentVersion(documentId);
  const archived = await versions()
    .find({ documentId })
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
}

/** Nội dung một version cụ thể (hiện tại lấy từ `documents`, cũ lấy từ kho). */
export async function getVersionContent(documentId: string, version: number) {
  const current = await getCurrentVersion(documentId);
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
  const archived = await versions().findOne({ documentId, version });
  if (!archived) return { found: false };
  return {
    found: true,
    documentId,
    version,
    source: archived.source as string,
    content: archived.content as string,
    isLatest: false,
  };
}
