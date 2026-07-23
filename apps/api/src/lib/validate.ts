import type { ZodType } from "zod";
import { BadRequestError } from "./errors";

/**
 * Parse dữ liệu ở BIÊN (body/params) bằng Zod. Sai → BadRequestError (400) để
 * error handler tập trung trả về đúng status thay vì rơi vào 500.
 */
export function parseOrBadRequest<T>(schema: ZodType<T>, data: unknown): T {
  const result = schema.safeParse(data);
  if (!result.success) {
    const message = result.error.issues.map((i) => i.message).join("; ");
    throw new BadRequestError(message || "Dữ liệu không hợp lệ");
  }
  return result.data;
}
