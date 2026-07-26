import type { preHandlerAsyncHookHandler } from "fastify";
import jwt from "jsonwebtoken";
import { config } from "../../config";
import { UnauthorizedError } from "../errors/app-errors";
import type { JwtPayload } from "../interfaces/auth-context";

/**
 * Guard xác thực — dùng làm `preHandler` cho các route cần auth.
 *
 * Yêu cầu: `@fastify/cookie` đã được register trước khi dùng guard này.
 *
 * Luồng xử lý:
 * 1. Đọc `access_token` từ cookie (do @fastify/cookie parse).
 * 2. Verify JWT bằng secret.
 * 3. Gắn `req.tenantId` và `req.user` để các tầng dưới dùng.
 * 4. Ném `UnauthorizedError` nếu thiếu token hoặc token không hợp lệ.
 *
 * Cách dùng:
 * ```ts
 * app.get('/api/protected', { preHandler: [authGuard] }, handler);
 * ```
 */
export const authGuard: preHandlerAsyncHookHandler = async (req) => {
  const token = req.cookies?.access_token;
  if (!token) {
    throw new UnauthorizedError("Thiếu access_token trong cookie.");
  }

  let payload: JwtPayload;
  try {
    payload = jwt.verify(token, config.JWT_SECRET) as JwtPayload;
  } catch {
    throw new UnauthorizedError("Token không hợp lệ hoặc đã hết hạn.");
  }

  req.tenantId = payload.sub;
  req.user = payload;
};
