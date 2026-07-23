/**
 * Lỗi mang theo HTTP status, để controller chỉ cần `throw` và error handler
 * tập trung (middleware) map sang đúng status — không try/catch rải rác.
 */
export class HttpError extends Error {
  constructor(
    public statusCode: number,
    message: string,
    // code máy-đọc-được để client phân biệt loại lỗi (đi kèm trong response).
    public code: string = "HTTP_ERROR",
  ) {
    super(message);
    this.name = "HttpError";
  }
}

export class BadRequestError extends HttpError {
  constructor(message: string) {
    super(400, message, "BAD_REQUEST");
    this.name = "BadRequestError";
  }
}

export class NotFoundError extends HttpError {
  constructor(message: string) {
    super(404, message, "NOT_FOUND");
    this.name = "NotFoundError";
  }
}
