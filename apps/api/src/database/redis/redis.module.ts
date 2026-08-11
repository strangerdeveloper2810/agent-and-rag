/**
 * Redis module — connection + typed helpers.
 *
 * Dùng cho:
 * - Rate limiting (thay in-memory @fastify/rate-limit)
 * - Embedding cache (Voyage API — deterministic)
 * - LLM chat response cache
 * - Tool call result cache
 */
import { Redis } from "ioredis";

let redis: Redis | null = null;

export interface RedisConfig {
  url: string; // redis://user:pass@host:6379/0
  maxRetriesPerRequest?: number;
}

/** Khởi tạo Redis connection. Idempotent. */
export const initRedis = async (config: RedisConfig): Promise<Redis> => {
  if (redis) return redis;
  redis = new Redis(config.url, {
    maxRetriesPerRequest: config.maxRetriesPerRequest ?? 3,
    lazyConnect: true,
  });
  await redis.connect();
  await redis.ping();
  return redis;
};

/** Lấy Redis instance đã khởi tạo. Throw nếu chưa gọi initRedis(). */
export const getRedis = (): Redis => {
  if (!redis)
    throw new Error("Redis not initialized. Call initRedis() first.");
  return redis;
};

/** Đóng kết nối Redis (graceful shutdown). */
export const closeRedis = async (): Promise<void> => {
  await redis?.quit();
  redis = null;
};

// ── Typed helpers ──

/** Lưu JSON vào Redis với optional TTL (giây). */
export const cacheSet = async <T>(
  key: string,
  value: T,
  ttlSeconds?: number,
): Promise<void> => {
  const r = getRedis();
  const raw = JSON.stringify(value);
  if (ttlSeconds) {
    await r.setex(key, ttlSeconds, raw);
  } else {
    await r.set(key, raw);
  }
};

/** Đọc JSON từ Redis. Trả về null nếu key không tồn tại. */
export const cacheGet = async <T>(key: string): Promise<T | null> => {
  const r = getRedis();
  const raw = await r.get(key);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as T;
  } catch {
    return null;
  }
};

/** Xoá 1 key khỏi cache. */
export const cacheDel = async (key: string): Promise<void> => {
  await getRedis().del(key);
};

/** Xoá tất cả key khớp pattern (dùng SCAN để không block). */
export const cacheDelPattern = async (pattern: string): Promise<void> => {
  const r = getRedis();
  const stream = r.scanStream({ match: pattern, count: 100 });
  for await (const keys of stream) {
    if (keys.length > 0) await r.del(...(keys as string[]));
  }
};

/** Tạo cache key có namespace, tự động lowercase + trim. */
export const cacheKey = (namespace: string, ...parts: string[]): string =>
  `jarvis:${namespace}:${parts.map((p) => p.toLowerCase().trim()).join(":")}`;
