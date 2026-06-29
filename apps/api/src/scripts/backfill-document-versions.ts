import { ObjectId } from "mongodb";
import { connectMongo, closeMongo } from "../lib/mongo";

/**
 * Migration một lần: gán documentId + version=1 cho các chunk tài liệu CŨ
 * (upload trước khi có versioning nên thiếu 2 field này).
 *
 * Mỗi `source` (tên file) cũ = 1 tài liệu logic → 1 documentId mới.
 * Chunk đã có documentId (bản mới) được bỏ qua.
 *
 * Chạy: pnpm migrate:docs
 */
async function main() {
  const db = await connectMongo();
  const col = db.collection("documents");

  const sources = await col.distinct("source", {
    documentId: { $exists: false },
  });

  if (sources.length === 0) {
    console.log("Không có tài liệu cũ cần migrate. ✓");
    return;
  }

  for (const source of sources) {
    const documentId = new ObjectId().toHexString();
    const res = await col.updateMany(
      { source, documentId: { $exists: false } },
      { $set: { documentId, version: 1 } },
    );
    console.log(`• ${source} → documentId=${documentId} (${res.modifiedCount} chunk)`);
  }

  console.log(`Xong. Đã migrate ${sources.length} tài liệu.`);
}

main()
  .catch((err) => {
    console.error("Migration lỗi:", err);
    process.exitCode = 1;
  })
  .finally(() => closeMongo());
