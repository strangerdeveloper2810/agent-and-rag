/**
 * Lỗi mang theo HTTP status, để controller chỉ cần `throw` và error handler
 * tập trung (middleware) map sang đúng status — không try/catch rải rác.
 */
export class HttpError extends Error {
  constructor(
    public statusCode: number,
    message: string,
    /** Mã lỗi máy-đọc-được để client phân biệt loại lỗi trong response body. */
    public code: string = "HTTP_ERROR",
  ) {
    super(message);
    this.name = "HttpError";
  }
}

/** 400 — Client gửi dữ liệu không hợp lệ (validate thất bại, thiếu field, sai kiểu). */
export class BadRequestError extends HttpError {
  constructor(message: string) {
    super(400, message, "BAD_REQUEST");
    this.name = "BadRequestError";
  }
}

/** 404 — Tài nguyên không tồn tại (conversation, document, task). */
export class NotFoundError extends HttpError {
  constructor(message: string) {
    super(404, message, "NOT_FOUND");
    this.name = "NotFoundError";
  }
}

/**
 * 429 — Vượt giới hạn tốc độ gọi API.
 * Dùng cho cả rate-limit toàn cục (Fastify plugin) và rate-limit từ provider bên ngoài.
 */
export class RateLimitError extends HttpError {
  constructor(
    message = "Quá nhiều request. Vui lòng chậm lại và thử lại sau.",
    /** Số giây khuyến nghị client đợi trước khi retry (đưa vào header Retry-After). */
    public retryAfterSeconds: number = 60,
  ) {
    super(429, message, "RATE_LIMITED");
    this.name = "RateLimitError";
  }
}

/**
 * 502 — Go agent runtime không phản hồi (health check thất bại hoặc connection refused).
 * Gateway trả 502 kèm Retry-After để client biết thử lại sau.
 */
export class AgentUnavailableError extends HttpError {
  constructor(
    message = "AI agent hiện không khả dụng. Vui lòng thử lại sau.",
    /** Số giây khuyến nghị client đợi trước khi retry. */
    public retryAfterSeconds: number = 30,
  ) {
    super(502, message, "AGENT_UNAVAILABLE");
    this.name = "AgentUnavailableError";
  }
}

/**
 * 504 — Go agent mất quá nhiều thời gian phản hồi (vượt AGENT_GO_TIMEOUT).
 */
export class AgentTimeoutError extends HttpError {
  constructor(
    message = "AI agent phản hồi quá chậm. Vui lòng thử lại.",
    /** Thời gian timeout đã cấu hình (ms), để client biết backend đã chờ bao lâu. */
    public timeoutMs: number = 120_000,
  ) {
    super(504, message, "AGENT_TIMEOUT");
    this.name = "AgentTimeoutError";
  }
}
