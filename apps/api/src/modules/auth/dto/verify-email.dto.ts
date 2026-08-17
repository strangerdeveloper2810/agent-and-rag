import { z } from "zod";

export const verifyEmailSchema = z.object({
  email: z.string().email("Email không hợp lệ"),
  otp: z.string().length(6, "OTP phải gồm 6 chữ số"),
});

export type VerifyEmailInput = z.infer<typeof verifyEmailSchema>;
