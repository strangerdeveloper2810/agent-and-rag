/**
 * Chat Response Cache — cache LLM chat responses by exact prompt match.
 *
 * LLM không deterministic (temperature > 0), nhưng với cùng model + temperature
 * + messages, response thường tương tự. Cache exact-match để tránh gọi API
 * trùng lặp trong thời gian ngắn.
 *
 * Key = md5(model + temperature + JSON(messages))
 * TTL = 1 giờ (đủ lâu để tránh trùng lặp, đủ ngắn để không stale)
 */
import { createHash } from "crypto";
import {
  cacheGet,
  cacheSet,
  cacheKey,
} from "../../database/redis/redis.module";

const CHAT_TTL = 3600; // 1 giờ

const hash = (input: string): string =>
  createHash("md5").update(input).digest("hex");

interface ChatCacheEntry {
  response: string;
  model: string;
  temperature: number;
  cachedAt: string;
}

interface ChatCacheInput {
  model: string;
  temperature: number;
  messages: Record<string, unknown>[];
}

const buildKey = (input: ChatCacheInput): string => {
  const raw = JSON.stringify({
    model: input.model,
    temp: input.temperature,
    msgs: input.messages,
  });
  return cacheKey("chat", hash(raw));
};

/** Lấy cached chat response. null = cache miss. */
export const getChatCache = async (
  input: ChatCacheInput,
): Promise<string | null> => {
  const entry = await cacheGet<ChatCacheEntry>(buildKey(input));
  return entry?.response ?? null;
};

/** Lưu chat response vào cache. */
export const setChatCache = async (
  input: ChatCacheInput,
  response: string,
): Promise<void> => {
  const entry: ChatCacheEntry = {
    response,
    model: input.model,
    temperature: input.temperature,
    cachedAt: new Date().toISOString(),
  };
  await cacheSet(buildKey(input), entry, CHAT_TTL);
};
