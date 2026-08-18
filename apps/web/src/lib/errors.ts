import type { TFunction } from "i18next";
import { ApiError } from "./http";

/**
 * Dịch lỗi backend sang chuỗi theo ngôn ngữ UI hiện tại, tra theo `ApiError.code`
 * trong namespace `errors`. Không bao giờ hiện `err.message` thô từ backend
 * (tránh lẫn ngôn ngữ khi backend luôn trả tiếng Việt bất kể locale UI).
 *
 * Code không có trong `errors.json` (hoặc `err` không phải ApiError) → dùng
 * `fallback` do caller cung cấp (thường là 1 chuỗi t() theo ngữ cảnh trang đó).
 */
export const translateApiError = (
  err: unknown,
  t: TFunction,
  fallback: string,
  values?: Record<string, unknown>,
): string => {
  if (err instanceof ApiError && err.code) {
    return t(`errors:${err.code}`, { ...values, defaultValue: fallback });
  }
  return fallback;
};
