/**
 * Hệ thống lỗi lớp chung (common) dùng trong BFF refactor.
 *
 * KHÁC BIỆT với `lib/errors.ts` (HttpError hierarchy):
 * - `lib/errors.ts` là tầng "legacy" dùng chung cho controller hiện tại,
 *   dựa trên HttpError với statusCode + code.
 * - `app-errors.ts` là tầng "mới" cho BFF, dựa trên AppError với cấu trúc
 *   tương tự NHƯNG phân cấp rõ ràng hơn theo miền lỗi (auth, validation, ...).
 * - HAI TẦNG CÙNG TỒN TẠI — error filter chung sẽ bắt cả hai.
 * - KHÔNG XOÁ HOẶC SỬA `lib/errors.ts`.
 */

/**
 * Lỗi ứng dụng cơ sở.
 * Controller throw subclass; error filter tập trung map sang HTTP response.
 */
export class AppError extends Error {
  constructor(
    message: string,
    public statusCode: number = 500,
    /** Mã máy-đọc-được để client phân biệt loại lỗi trong response body. */
    public code: string = "INTERNAL_ERROR",
  ) {
    super(message);
    this.name = "AppError";
  }
}

/** 401 — Không có token hoặc token không hợp lệ / hết hạn. */
export class UnauthorizedError extends AppError {
  constructor(message = "Chưa xác thực. Vui lòng đăng nhập.") {
    super(message, 401, "UNAUTHORIZED");
    this.name = "UnauthorizedError";
  }
}

/** 403 — Có token nhưng không đủ quyền truy cập tài nguyên. */
export class ForbiddenError extends AppError {
  constructor(message = "Bạn không có quyền truy cập tài nguyên này.") {
    super(message, 403, "FORBIDDEN");
    this.name = "ForbiddenError";
  }
}

/** 404 — Tài nguyên không tồn tại. */
export class NotFoundError extends AppError {
  constructor(message = "Không tìm thấy tài nguyên.") {
    super(message, 404, "NOT_FOUND");
    this.name = "NotFoundError";
  }
}

/** 409 — Xung đột trạng thái (vd tạo resource đã tồn tại). */
export class ConflictError extends AppError {
  constructor(message = "Xung đột dữ liệu.") {
    super(message, 409, "CONFLICT");
    this.name = "ConflictError";
  }
}

/**
 * 422 — Dữ liệu đầu vào không hợp lệ (validate thất bại).
 * Mang theo `fieldErrors` map từng field -> danh sách lỗi để client
 * hiển thị lỗi theo field (vd highlight ô input sai).
 */
export class ValidationError extends AppError {
  public readonly fieldErrors: Record<string, string[]>;

  constructor(fieldErrors: Record<string, string[]>) {
    const totalIssues = Object.values(fieldErrors).flat().length;
    super(
      `Dữ liệu không hợp lệ (${totalIssues} lỗi).`,
      422,
      "VALIDATION_ERROR",
    );
    this.name = "ValidationError";
    this.fieldErrors = fieldErrors;
  }
}
