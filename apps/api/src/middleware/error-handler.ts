import type { FastifyInstance, FastifyReply } from "fastify";
import {
  HttpError,
  AgentUnavailableError,
  AgentTimeoutError,
  RateLimitError,
} from "../lib/errors";
import { VoyageError } from "../lib/voyage";
import {
  UnsupportedFileError,
  EmptyContentError,
} from "../modules/documents/extract";

/**
 * Error handler tập trung (tầng "middleware").
 * Controller chỉ cần `throw` lỗi có kiểu; chỗ này map sang HTTP status + body
 * CHUẨN HOÁ `{ error, code }` (code máy-đọc-được). Gọi 1 lần trong buildApp().
 *
 * Thứ tự check QUAN TRỌNG: các lớp con của HttpError (AgentUnavailable,
 * AgentTimeout, RateLimit) phải được check TRƯỚC HttpError để có cơ hội
 * set header riêng (vd Retry-After).
 */
export function registerErrorHandler(app: FastifyInstance) {
  const send = (
    reply: FastifyReply,
    status: number,
    error: string,
    code: string,
  ) => reply.code(status).send({ error, code });

  app.setErrorHandler((err, req, reply) => {
    // Agent unavailable — 502 + Retry-After header.
    if (err instanceof AgentUnavailableError) {
      reply.header("Retry-After", err.retryAfterSeconds);
      return send(reply, 502, err.message, err.code);
    }

    // Agent timeout — 504.
    if (err instanceof AgentTimeoutError) {
      return send(reply, 504, err.message, err.code);
    }

    // Rate limit — 429 + Retry-After header.
    if (err instanceof RateLimitError) {
      reply.header("Retry-After", err.retryAfterSeconds);
      return send(reply, 429, err.message, err.code);
    }

    // Các lỗi HttpError còn lại (BadRequest, NotFound, ...).
    if (err instanceof HttpError) {
      return send(reply, err.statusCode, err.message, err.code);
    }

    // Định dạng file không hỗ trợ (415)
    if (err instanceof UnsupportedFileError) {
      return send(reply, 415, err.message, "UNSUPPORTED_FILE");
    }

    // Trích xuất text rỗng, vd PDF scan (422)
    if (err instanceof EmptyContentError) {
      return send(reply, 422, err.message, "EMPTY_CONTENT");
    }

    // Voyage rate limit (429)
    if (err instanceof VoyageError && err.status === 429) {
      return send(
        reply,
        429,
        "Voyage đang giới hạn tốc độ (free tier: 3 request/phút, 10K token/phút). " +
          "Thêm payment method tại dashboard.voyageai.com để nâng giới hạn (vẫn miễn phí 200M token), " +
          "hoặc đợi ~1 phút rồi thử lại với file nhỏ hơn.",
        "RATE_LIMITED",
      );
    }

    // File vượt limits.fileSize của multipart (413)
    if ((err as { code?: string }).code === "FST_REQ_FILE_TOO_LARGE") {
      return send(reply, 413, "File quá lớn (tối đa 25MB).", "FILE_TOO_LARGE");
    }

    // Lỗi validate/route của Fastify (có statusCode < 500)
    const status = (err as { statusCode?: number }).statusCode;
    if (typeof status === "number" && status < 500) {
      return send(reply, status, (err as Error).message, "BAD_REQUEST");
    }

    // Còn lại = lỗi máy chủ (500)
    req.log.error(err);
    return send(reply, 500, "Lỗi máy chủ.", "INTERNAL");
  });
}
