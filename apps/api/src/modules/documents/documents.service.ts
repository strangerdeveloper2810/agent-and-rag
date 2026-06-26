import { chunkText } from "./chunk";
import { embed } from "../../lib/voyage";
import { insertChunks, type DocChunk } from "./documents.repository";

/**
 * Pipeline nạp tài liệu: text thô → chunk → embed (Voyage) → lưu vào Mongo.
 */
export async function ingestDocument(source: string, content: string) {
  const chunks = await chunkText(content);
  const embeddings = await embed(chunks, "document");
  const now = new Date();
  const docs: DocChunk[] = chunks.map((text, i) => ({
    source,
    chunkIndex: i,
    text,
    embedding: embeddings[i],
    createdAt: now,
  }));
  await insertChunks(docs);
  return { source, chunks: docs.length };
}
