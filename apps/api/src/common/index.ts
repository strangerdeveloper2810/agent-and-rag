/**
 * Common layer — điểm vào duy nhất cho toàn bộ BFF common utilities.
 *
 * Re-export mọi thứ từ các sub-module để consumer chỉ cần import từ `common`.
 *
 * Cách dùng:
 * ```ts
 * import { AppError, UnauthorizedError, validate, authGuard, adminGuard, registerErrorFilter } from '../common';
 * ```
 */

// ---------------------------------------------------------------------------
// Errors
// ---------------------------------------------------------------------------
export {
  AppError,
  UnauthorizedError,
  ForbiddenError,
  NotFoundError,
  ConflictError,
  ValidationError,
} from "./errors/app-errors";

// ---------------------------------------------------------------------------
// Interfaces (chỉ export type để tránh import runtime không cần thiết)
// ---------------------------------------------------------------------------
export type { JwtPayload } from "./interfaces/auth-context";

// ---------------------------------------------------------------------------
// Guards
// ---------------------------------------------------------------------------
export { authGuard } from "./guards/auth.guard";
export { adminGuard } from "./guards/admin.guard";

// ---------------------------------------------------------------------------
// Filters
// ---------------------------------------------------------------------------
export { registerErrorFilter } from "./filters/error.filter";

// ---------------------------------------------------------------------------
// Pipes
// ---------------------------------------------------------------------------
export { validate } from "./pipes/validation.pipe";
