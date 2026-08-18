/**
 * Client-side validation schemas dùng chung giữa react-hook-form và zod.
 *
 * Các schema này khớp với DTO bên API (apps/api/src/modules/auth/dto/)
 * nhưng message lỗi viết cho end-user (tiếng Việt, thân thiện).
 */

import { z } from "zod";

export const loginSchema = z.object({
  email: z
    .string()
    .min(1, "Vui lòng nhập email.")
    .email("Email không đúng định dạng."),
  password: z.string().min(1, "Vui lòng nhập mật khẩu."),
});

export const registerSchema = z.object({
  name: z
    .string()
    .min(1, "Vui lòng nhập họ tên.")
    .max(100, "Họ tên không được vượt quá 100 ký tự."),
  email: z
    .string()
    .min(1, "Vui lòng nhập email.")
    .email("Email không đúng định dạng."),
  password: z.string().min(8, "Mật khẩu phải có ít nhất 8 ký tự."),
});

export const verifyEmailOtpSchema = z.object({
  otp: z
    .string()
    .length(6, "Mã OTP phải gồm 6 chữ số.")
    .regex(/^\d{6}$/, "Mã OTP chỉ gồm chữ số."),
});

export const forgotPasswordSchema = z.object({
  email: z
    .string()
    .min(1, "Vui lòng nhập email.")
    .email("Email không đúng định dạng."),
});

export const resetPasswordSchema = z
  .object({
    otp: z
      .string()
      .length(6, "Mã OTP phải gồm 6 chữ số.")
      .regex(/^\d{6}$/, "Mã OTP chỉ gồm chữ số."),
    newPassword: z.string().min(8, "Mật khẩu mới phải từ 8 ký tự trở lên."),
    confirmPassword: z.string().min(1, "Vui lòng xác nhận mật khẩu."),
  })
  .refine((data) => data.newPassword === data.confirmPassword, {
    message: "Mật khẩu xác nhận không khớp.",
    path: ["confirmPassword"],
  });

export type LoginFormValues = z.infer<typeof loginSchema>;
export type RegisterFormValues = z.infer<typeof registerSchema>;
export type VerifyEmailOtpFormValues = z.infer<typeof verifyEmailOtpSchema>;
export type ForgotPasswordFormValues = z.infer<typeof forgotPasswordSchema>;
export type ResetPasswordFormValues = z.infer<typeof resetPasswordSchema>;

// ── Composer / Chat input ──

/** Các pattern XSS/HTML injection phổ biến cần chặn ở client. */
const XSS_PATTERN =
  /<script[\s>]|<\/script>|javascript\s*:|on\w+\s*=\s*["']|onerror\s*=|onload\s*=|onclick\s*=|eval\s*\(|document\.cookie|document\.write|<iframe|<object|<embed/i;

/**
 * Các pattern prompt injection phổ biến nhằm chiếm quyền điều khiển LLM.
 * Không chặn SQL/code hợp lệ — chỉ chặn các câu lệnh cố tình override system prompt.
 */
const PROMPT_INJECTION_PATTERN =
  /(?:^|\b)(ignore\s+(all\s+)?(previous|prior|above)\s+(instructions?|prompts?|messages?|conversations?|text|context))|(?:you\s+are\s+now\s+(DAN|jailbreak|a\s+different))|(?:forget\s+(everything|all\s+(previous|prior)\s+instructions?))|(?:system\s*:\s*(override|prompt|instruction|you\s+are))|(?:print\s+(your|the)\s+(system\s+)?(instructions?|prompts?|rules?))|(?:reveal\s+(your|the)\s+(system\s+)?(instructions?|prompts?|rules?))|(?:what\s+(is|are)\s+(your|the)\s+(system\s+)?(instructions?|prompts?|rules?)\??\s*$)|(?:new\s+instructions?\s*:)|(?:from\s+now\s+on\s+you\s+are)/i;

export const composerSchema = z.object({
  content: z
    .string()
    .max(4000, "Tin nhắn quá dài (tối đa 4000 ký tự).")
    .refine((val) => !XSS_PATTERN.test(val), {
      message: "Tin nhắn chứa nội dung không hợp lệ.",
    })
    .refine((val) => !PROMPT_INJECTION_PATTERN.test(val), {
      message: "Tin nhắn chứa nội dung không được phép.",
    }),
});

export type ComposerFormValues = z.infer<typeof composerSchema>;

/**
 * Validate input chat trước khi gửi.
 * Trả về `{ valid: true, content }` hoặc `{ valid: false, error }`.
 * Dùng trong ChatPage.send() và Composer.handleKeyDown.
 */
export const validateComposerInput = (
  input: string,
): { valid: true; content: string } | { valid: false; error: string } => {
  const result = composerSchema.safeParse({ content: input });
  if (!result.success) {
    const firstIssue = result.error.issues[0];
    return { valid: false, error: firstIssue.message };
  }
  return { valid: true, content: result.data.content };
};
