import type { FastifyRequest } from "fastify";
import jwt from "jsonwebtoken";
import { config } from "../../config";
import type { JwtPayload } from "../interfaces/auth-context";

/**
 * Sinh khoá cho @fastify/rate-limit.
 *
 * Mặc định plugin khoá theo `req.ip`, và điều đó SAI với kiến trúc này theo hai
 * cách khác nhau:
 *
 * 1. Trước fix, nginx trong container không chuyển tiếp `X-Forwarded-For` nên
 *    `req.ip` = IP của nginx container với MỌI request → cả Internet dùng chung
 *    một bucket. Hạn mức "20 chat/phút" hoá ra là 20 cho toàn hệ thống: đo trên
 *    production, 60 request từ 1 máy chỉ có 19 lọt, và user thứ 21 trong phút đó
 *    bị 429 dù chưa gửi gì. Xem docs/2026-08-18-loadtest-production-report.md.
 *
 * 2. Ngay cả khi IP đã đúng, khoá theo IP vẫn không đúng ý "mỗi người N tin/phút":
 *    cả một office/trường học sau NAT dùng chung một IP, còn một người đổi mạng
 *    (wifi → 4G) lại được cấp thêm hạn mức mới.
 *
 * Nên: user đã đăng nhập → khoá theo `sub` trong JWT. Chưa đăng nhập (login,
 * register, refresh) → khoá theo IP, vì đó chính là thứ cần chống brute-force.
 *
 * Token được VERIFY chứ không chỉ decode: nếu chỉ decode, ai cũng tự bơm một
 * `sub` bất kỳ vào cookie để nhận bucket mới và đi vòng qua rate limit.
 */
export const rateLimitKeyGenerator = (req: FastifyRequest): string => {
  const token = req.cookies?.access_token;
  if (token) {
    try {
      const payload = jwt.verify(token, config.JWT_SECRET) as JwtPayload;
      if (payload?.sub) return `user:${payload.sub}`;
    } catch {
      // Token hỏng/hết hạn → rơi về IP. Không ném lỗi ở đây: rate limit chạy
      // trong onRequest, trước authGuard; việc từ chối token là việc của guard,
      // hook này chỉ cần chọn được một khoá ổn định.
    }
  }
  return `ip:${req.ip}`;
};
