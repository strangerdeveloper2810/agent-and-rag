import type { preHandlerAsyncHookHandler } from "fastify";
import { ForbiddenError } from "../errors/app-errors";

/**
 * Guard phân quyền admin — dùng làm `preHandler` cho route chỉ admin được gọi.
 *
 * YÊU CẦU: `authGuard` phải chạy TRƯỚC guard này (để `req.user` đã có dữ liệu).
 * Nếu không có `req.user` (guard chạy sai thứ tự), vẫn ném ForbiddenError.
 *
 * Dùng async style: throw lỗi trực tiếp để Fastify bắt và chuyển
 * cho error handler tập trung.
 *
 * Cách dùng:
 * ```ts
 * app.get('/api/admin/users', { preHandler: [authGuard, adminGuard] }, handler);
 * ```
 */
export const adminGuard: preHandlerAsyncHookHandler = async (req) => {
  if (req.user?.role !== "admin") {
    throw new ForbiddenError(
      "Chỉ quản trị viên (admin) mới có quyền truy cập tài nguyên này.",
    );
  }
};
