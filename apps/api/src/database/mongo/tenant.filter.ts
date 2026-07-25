import type { FastifyRequest } from "fastify";

/**
 * Tạo Mongo filter tự động gắn tenantId từ request context.
 *
 * Dùng trong mọi repository query để đảm bảo cách ly dữ liệu giữa các tenant.
 * Nếu request chưa có tenantId (giai đoạn chuyển tiếp), trả về extra filter
 * nguyên bản — không ép buộc filter tenant để tránh query trả về rỗng.
 *
 * @example
 * ```ts
 * const docs = await collections.conversations()
 *   .find(tenantFilter(req, { status: "active" }))
 *   .toArray();
 * ```
 */
export function tenantFilter(
  req: FastifyRequest,
  extra: Record<string, unknown> = {},
): Record<string, unknown> {
  const tid = (req as unknown as Record<string, unknown>).tenantId;
  if (!tid) return { ...extra };
  return { tenantId: tid as string, ...extra };
}
