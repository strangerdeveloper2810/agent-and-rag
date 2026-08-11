import type { FastifyInstance, FastifyReply } from "fastify";
import {
  AppError,
  UnauthorizedError,
  ForbiddenError,
  NotFoundError as AppNotFoundError,
  ConflictError,
  ValidationError,
  ServiceUnavailableError,
} from "../errors/app-errors";
import {
  HttpError,
  AgentUnavailableError,
  AgentTimeoutError,
  RateLimitError,
} from "../../lib/errors";
import { VoyageError } from "../../lib/voyage";
import {
  UnsupportedFileError,
  EmptyContentError,
} from "../../modules/documents/extract";

/**
 * Error filter tập trung (kế thừa + nâng cấp từ `middleware/error-handler.ts`).
 *
 * Xử lý CẢ HAI tầng lỗi:
 *  - AppError (common) — mới, dùng cho BFF refactor.
 *  - HttpError (lib)   — legacy, controller hiện tại vẫn dùng.
 *
 * Controller chỉ cần `throw`; chỗ này map sang HTTP status + body
 * CHUẨN HOÁ `{ error, code }` (code máy-đọc-được).
 *
 * THỨ TỰ CHECK QUAN TRỌNG:
 *  1. ValidationError (có fieldErrors) — trước AppError base.
 *  2. Các AppError còn lại (Unauthorized, Forbidden, NotFound, Conflict).
 *  3. AgentUnavailableError / AgentTimeoutError / RateLimitError (cần set header).
 *  4. HttpError base (BadRequest, NotFound-legacy, ...).
 *  5. Lỗi miền (UnsupportedFile, EmptyContent, Voyage).
 *  6. Lỗi Fastify (FST_*, validation).
 *  7. Fallback 500.
 */
export const registerErrorFilter = (app: FastifyInstance): void => {
  /** Gửi response lỗi chuẩn hoá { error, code, ...extras }. */
  const send = (
    reply: FastifyReply,
    status: number,
    error: string,
    code: string,
    extras?: Record<string, unknown>,
  ) => reply.code(status).send({ error, code, ...extras });

  app.setErrorHandler((err, req, reply) => {
    // ---- 1. ValidationError (có fieldErrors → trả về cho client hiển thị) ----
    if (err instanceof ValidationError) {
      return send(reply, 422, err.message, "VALIDATION_ERROR", {
        fieldErrors: err.fieldErrors,
      });
    }

    // ---- 2. AppError: Unauthorized / Forbidden / NotFound / Conflict ----
    if (err instanceof UnauthorizedError) {
      return send(reply, 401, err.message, "UNAUTHORIZED");
    }

    if (err instanceof ForbiddenError) {
      return send(reply, 403, err.message, "FORBIDDEN");
    }

    if (err instanceof AppNotFoundError) {
      return send(reply, 404, err.message, "NOT_FOUND");
    }

    if (err instanceof ConflictError) {
      return send(reply, 409, err.message, "CONFLICT");
    }

    if (err instanceof ServiceUnavailableError) {
      return send(reply, 503, err.message, "SERVICE_UNAVAILABLE");
    }

    if (err instanceof AppError) {
      // Bắt các AppError còn lại (có thể mở rộng sau này).
      return send(reply, err.statusCode, err.message, err.code);
    }

    // ---- 3. Agent unavailable — 502 + Retry-After header ----
    if (err instanceof AgentUnavailableError) {
      reply.header("Retry-After", err.retryAfterSeconds);
      return send(reply, 502, err.message, err.code);
    }

    // ---- 4. Agent timeout — 504 ----
    if (err instanceof AgentTimeoutError) {
      return send(reply, 504, err.message, err.code);
    }

    // ---- 5. Rate limit — 429 + Retry-After header ----
    if (err instanceof RateLimitError) {
      reply.header("Retry-After", err.retryAfterSeconds);
      return send(reply, 429, err.message, err.code);
    }

    // ---- 6. Các HttpError còn lại (BadRequest, NotFound-legacy, ...) ----
    if (err instanceof HttpError) {
      return send(reply, err.statusCode, err.message, err.code);
    }

    // ---- 7. Định dạng file không hỗ trợ — 415 ----
    if (err instanceof UnsupportedFileError) {
      return send(reply, 415, err.message, "UNSUPPORTED_FILE");
    }

    // ---- 8. Trích xuất text rỗng (PDF scan) — 422 ----
    if (err instanceof EmptyContentError) {
      return send(reply, 422, err.message, "EMPTY_CONTENT");
    }

    // ---- 9. Voyage rate limit — 429 ----
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

    // ---- 10. File vượt limits.fileSize của multipart — 413 ----
    if ((err as { code?: string }).code === "FST_REQ_FILE_TOO_LARGE") {
      return send(reply, 413, "File quá lớn (tối đa 25MB).", "FILE_TOO_LARGE");
    }

    // ---- 11. Lỗi validate/route của Fastify (có statusCode < 500) ----
    const status = (err as { statusCode?: number }).statusCode;
    if (typeof status === "number" && status < 500) {
      return send(reply, status, (err as Error).message, "BAD_REQUEST");
    }

    // ---- 12. Còn lại = lỗi máy chủ — 500 ----
    req.log.error(err);
    return send(reply, 500, "Lỗi máy chủ.", "INTERNAL");
  });
};
