import { z } from "zod";

export const resetPasswordSchema = z.object({
  email: z.string().email("Email không hợp lệ"),
  otp: z.string().length(6, "OTP phải gồm 6 chữ số"),
  newPassword: z.string().min(8, "Mật khẩu phải từ 8 ký tự trở lên"),
});

export type ResetPasswordInput = z.infer<typeof resetPasswordSchema>;
