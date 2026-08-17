import { z } from "zod";

export const resendOtpSchema = z.object({
  email: z.string().email("Email không hợp lệ"),
});

export type ResendOtpInput = z.infer<typeof resendOtpSchema>;
