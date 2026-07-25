/**
 * Ngữ cảnh xác thực cho toàn bộ BFF layer.
 *
 * Sau khi auth guard xác thực JWT thành công, payload được gắn vào
 * `req.user` và `req.tenantId` để các tầng dưới (controller/service)
 * dùng mà không cần parse lại token.
 */

/** Payload của JWT token sau khi verify. */
export interface JwtPayload {
  /** ID của user (subject trong JWT). */
  sub: string;
  /** Email của user. */
  email: string;
  /** Vai trò phân quyền. */
  role: "user" | "admin";
}

// Mở rộng kiểu FastifyRequest để có tenantId và user.
// Các guard (auth, admin) sẽ set các field này sau khi xác thực.
declare module "fastify" {
  interface FastifyRequest {
    /** ID của tenant/user hiện tại (từ JWT payload.sub). */
    tenantId: string;
    /** Thông tin user đã xác thực từ JWT. */
    user: JwtPayload;
  }
}
