import type { FastifyInstance } from "fastify";
import { HttpError } from "../lib/errors";
import { VoyageError } from "../lib/voyage";
import {
  UnsupportedFileError,
  EmptyContentError,
} from "../modules/documents/extract";

/**
 * Error handler tập trung (tầng "middleware").
 * Controller chỉ cần `throw` lỗi có kiểu; chỗ này map sang HTTP status + message.
 * Gọi 1 lần trong buildApp().
 */
export function registerErrorHandler(app: FastifyInstance) {
  app.setErrorHandler((err, req, reply) => {
    // Lỗi có status tường minh (BadRequest, NotFound...)
    if (err instanceof HttpError) {
      return reply.code(err.statusCode).send({ error: err.message });
    }
    // Định dạng file không hỗ trợ
    if (err instanceof UnsupportedFileError) {
      return reply.code(415).send({ error: err.message });
    }
    // Trích ra text rỗng (vd PDF scan)
    if (err instanceof EmptyContentError) {
      return reply.code(422).send({ error: err.message });
    }
    // Voyage rate limit
    if (err instanceof VoyageError && err.status === 429) {
      return reply.code(429).send({
        error:
          "Voyage đang giới hạn tốc độ (free tier: 3 request/phút, 10K token/phút). " +
          "Thêm payment method tại dashboard.voyageai.com để nâng giới hạn (vẫn miễn phí 200M token), " +
          "hoặc đợi ~1 phút rồi thử lại với file nhỏ hơn.",
      });
    }
    // File vượt limits.fileSize của multipart
    if ((err as { code?: string }).code === "FST_REQ_FILE_TOO_LARGE") {
      return reply.code(413).send({ error: "File quá lớn (tối đa 25MB)." });
    }
    // Lỗi validate/route của Fastify (đã có statusCode < 500)
    const status = (err as { statusCode?: number }).statusCode;
    if (typeof status === "number" && status < 500) {
      return reply.code(status).send({ error: (err as Error).message });
    }
    // Còn lại = lỗi máy chủ
    req.log.error(err);
    return reply.code(500).send({ error: "Lỗi máy chủ." });
  });
}
