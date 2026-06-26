import { getDb } from "../../lib/mongo";

export type DocChunk = {
  source: string;
  chunkIndex: number;
  text: string;
  embedding: number[];
  createdAt: Date;
};

export async function insertChunks(chunks: DocChunk[]) {
  if (chunks.length === 0) return;
  await getDb().collection("documents").insertMany(chunks);
}

// Gom theo source để liệt kê tài liệu (mỗi tài liệu gồm nhiều chunk)
export async function listSources() {
  return getDb()
    .collection("documents")
    .aggregate([
      { $group: { _id: "$source", chunks: { $sum: 1 } } },
      { $project: { _id: 0, source: "$_id", chunks: 1 } },
    ])
    .toArray();
}

export async function deleteSource(source: string) {
  await getDb().collection("documents").deleteMany({ source });
}

/**
 * Tìm các chunk gần nghĩa nhất với câu hỏi bằng Atlas $vectorSearch.
 * - index: "vector_index" (phải khớp tên index tạo trên Atlas)
 * - numCandidates: số ứng viên Atlas quét (nhiều hơn limit để chính xác hơn)
 * - limit (k): số chunk trả về cuối cùng
 */
export async function searchSimilar(queryEmbedding: number[], k = 5) {
  return getDb()
    .collection("documents")
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
