/**
 * Embedding Cache — cache Voyage AI embedding results.
 *
 * Embedding là deterministic: cùng text + model + input_type → cùng vector.
 * Cache 30 ngày vì vector không đổi. Tiết kiệm cost API đáng kể khi:
 * - Upload lại cùng tài liệu
 * - Search với cùng query
 * - Re-index sau khi xoá nhầm
 */
import { createHash } from "crypto";
import { cacheGet, cacheSet, cacheKey } from "../../database/redis/redis.module";

const EMBED_TTL = 30 * 24 * 3600; // 30 ngày

const hash = (text: string): string =>
  createHash("md5").update(text).digest("hex");

interface EmbedCacheEntry {
  vectors: number[][];
  model: string;
  cachedAt: string;
}

/**
 * Lấy embedding từ cache nếu có. Trả về null nếu cache miss.
 * Mỗi text trong batch được cache riêng để tối đa hit rate.
 */
export const getEmbeddingCache = async (
  texts: string[],
  model: string,
  inputType: "document" | "query",
): Promise<{ vectors: number[][]; hits: boolean[] }> => {
  const vectors: number[][] = [];
  const hits: boolean[] = [];
  const misses: { text: string; idx: number }[] = [];

  for (let i = 0; i < texts.length; i++) {
    const key = cacheKey("embed", model, inputType, hash(texts[i]));
    const entry = await cacheGet<EmbedCacheEntry>(key);
    if (entry?.vectors?.length) {
      vectors[i] = entry.vectors[0];
      hits[i] = true;
    } else {
      hits[i] = false;
      misses.push({ text: texts[i], idx: i });
    }
  }

  return { vectors, hits };
};

/**
 * Lưu embedding vào cache.
 */
export const setEmbeddingCache = async (
  texts: string[],
  vectors: number[][],
  model: string,
  inputType: "document" | "query",
): Promise<void> => {
  const now = new Date().toISOString();
  const jobs = texts.map((text, i) => {
    const key = cacheKey("embed", model, inputType, hash(text));
    const entry: EmbedCacheEntry = {
      vectors: [vectors[i]],
      model,
      cachedAt: now,
    };
    return cacheSet(key, entry, EMBED_TTL);
  });
  await Promise.all(jobs);
};
