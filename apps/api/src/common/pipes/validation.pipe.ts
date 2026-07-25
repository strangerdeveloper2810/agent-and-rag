import type { ZodSchema } from "zod";
import { ValidationError } from "../errors/app-errors";

/**
 * Validate dữ liệu đầu vào bằng Zod schema.
 *
 * Khác với `lib/validate.ts` (`parseOrBadRequest` ném BadRequestError từ HttpError):
 * hàm này ném `ValidationError` từ AppError hierarchy (tầng mới), có kèm
 * `fieldErrors` để client hiển thị lỗi theo từng field.
 *
 * @param schema - Zod schema dùng để validate.
 * @param data   - Dữ liệu cần validate (body, query, params, ...).
 * @returns Dữ liệu đã parse + ép kiểu.
 * @throws  {ValidationError} nếu dữ liệu không hợp lệ.
 */
export const validate = <T>(schema: ZodSchema<T>, data: unknown): T => {
  const result = schema.safeParse(data);
  if (!result.success) {
    // Gom lỗi theo field path (dùng path.join('.') để tạo key).
    const fieldErrors: Record<string, string[]> = {};
    for (const issue of result.error.issues) {
      const key = issue.path.join(".") || "body";
      (fieldErrors[key] ??= []).push(issue.message);
    }
    throw new ValidationError(fieldErrors);
  }
  return result.data;
}
