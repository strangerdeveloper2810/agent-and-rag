import type { preHandlerAsyncHookHandler } from "fastify";

// ---------------------------------------------------------------------------
// TODO (auth phase): Cài dependency trước khi dùng guard này:
//   pnpm add jsonwebtoken
//   pnpm add -D @types/jsonwebtoken
//
// TODO (auth phase): Thêm JWT_SECRET vào envSchema trong config.ts:
//   JWT_SECRET: z.string().min(1, "JWT_SECRET is required"),
//
// Khi chưa cài jsonwebtoken, file này sẽ báo lỗi biên dịch — đó là CÓ CHỦ ĐÍCH.
// Khi triển khai auth, cài dependency + thêm JWT_SECRET vào config là hết lỗi.
// ---------------------------------------------------------------------------

// eslint-disable-next-line import/no-extraneous-dependencies
import jwt from "jsonwebtoken";
import { config } from "../../config";
import { UnauthorizedError } from "../errors/app-errors";
import type { JwtPayload } from "../interfaces/auth-context";

// Đọc JWT_SECRET từ config. Tạm cast về Record để tránh lỗi type khi
// JWT_SECRET chưa có trong envSchema (sẽ thêm sau khi triển khai auth).
const JWT_SECRET: string =
  (config as unknown as Record<string, string>).JWT_SECRET ??
  "CHANGE_ME_BEFORE_PRODUCTION";

/**
 * Trích cookie theo tên từ header `Cookie`.
 * Dùng parser thủ công thay vì `@fastify/cookie` để tránh thêm dependency
 * cho đến khi auth được triển khai chính thức.
 */
const getCookie = (cookieHeader: string, name: string): string | undefined => {
  for (const pair of cookieHeader.split(";")) {
    const [key, ...rest] = pair.trim().split("=");
    if (key === name) {
      return decodeURIComponent(rest.join("="));
    }
  }
  return undefined;
}

/**
 * Guard xác thực — dùng làm `preHandler` cho các route cần auth.
 *
 * Luồng xử lý:
 * 1. Đọc `access_token` từ cookie.
 * 2. Verify JWT bằng secret.
 * 3. Gắn `req.tenantId` và `req.user` để các tầng dưới dùng.
 * 4. Ném `UnauthorizedError` nếu thiếu token hoặc token không hợp lệ.
 *
 * Dùng async style: throw lỗi trực tiếp để Fastify bắt và chuyển
 * cho error handler tập trung (không cần gọi `done(err)`).
 *
 * Cách dùng:
 * ```ts
 * app.get('/api/protected', { preHandler: [authGuard] }, handler);
 * ```
 */
export const authGuard: preHandlerAsyncHookHandler = async (req) => {
  const cookieHeader = req.headers.cookie;
  if (!cookieHeader) {
    throw new UnauthorizedError("Thiếu cookie xác thực.");
  }

  const token = getCookie(cookieHeader, "access_token");
  if (!token) {
    throw new UnauthorizedError("Thiếu access_token trong cookie.");
  }

  // Verify JWT — ném JsonWebTokenError / TokenExpiredError nếu sai/hết hạn.
  let payload: JwtPayload;
  try {
    payload = jwt.verify(token, JWT_SECRET) as JwtPayload;
  } catch (err) {
    const message =
      err instanceof Error ? err.message : "Token không hợp lệ.";
    throw new UnauthorizedError(message);
  }

  // Gắn context xác thực vào request.
  req.tenantId = payload.sub;
  req.user = payload;
};
