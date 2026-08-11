/**
 * Tool Call Cache — cache kết quả tool call để tránh gọi lại trong thời gian ngắn.
 *
 * Chỉ cache tool call KHÔNG có side effect (idempotent):
 * - web_search, wikipedia_search: cache 5 phút
 * - get_document, list_documents: cache 1 phút (dữ liệu có thể thay đổi)
 * - KHÔNG cache: create_task, update_task, delete_* (có side effect)
 *
 * Key = md5(tool_name + JSON(args))
 * Mỗi tool có TTL riêng, config trong TOOL_TTL_MAP.
 */
import { createHash } from "crypto";
import { cacheGet, cacheSet, cacheKey } from "../../database/redis/redis.module";

const hash = (input: string): string =>
  createHash("md5").update(input).digest("hex");

// TTL riêng cho từng tool (giây)
const TOOL_TTL_MAP: Record<string, number> = {
  web_search: 300, // 5 phút
  wikipedia_search: 300,
  duckduckgo_search: 300,
  get_document: 60, // 1 phút — doc có thể được update
  list_documents: 60,
  get_task: 60,
  list_tasks: 60,
  get_memory: 300,
  search_memories: 300,
};

// Tool KHÔNG BAO GIỜ cache (có side effect hoặc real-time)
const NO_CACHE_TOOLS = new Set([
  "create_task",
  "update_task",
  "delete_task",
  "create_document",
  "delete_document",
  "upload_file",
  "send_message",
]);

const DEFAULT_TTL = 120; // 2 phút cho tool không có trong map

interface ToolCacheEntry {
  result: unknown;
  toolName: string;
  cachedAt: string;
  ttl: number;
}

const buildKey = (toolName: string, args: Record<string, unknown>): string => {
  const raw = JSON.stringify({ tool: toolName, args });
  return cacheKey("tool", toolName, hash(raw));
};

/**
 * Kiểm tra tool có được phép cache không.
 */
export const isToolCacheable = (toolName: string): boolean =>
  !NO_CACHE_TOOLS.has(toolName);

/**
 * Lấy cached tool result. null = cache miss hoặc tool không cacheable.
 */
export const getToolCache = async (
  toolName: string,
  args: Record<string, unknown>,
): Promise<unknown | null> => {
  if (!isToolCacheable(toolName)) return null;
  const entry = await cacheGet<ToolCacheEntry>(buildKey(toolName, args));
  return entry?.result ?? null;
};

/**
 * Lưu tool result vào cache.
 */
export const setToolCache = async (
  toolName: string,
  args: Record<string, unknown>,
  result: unknown,
): Promise<void> => {
  if (!isToolCacheable(toolName)) return;
  const ttl = TOOL_TTL_MAP[toolName] ?? DEFAULT_TTL;
  const entry: ToolCacheEntry = {
    result,
    toolName,
    cachedAt: new Date().toISOString(),
    ttl,
  };
  await cacheSet(buildKey(toolName, args), entry, ttl);
};
