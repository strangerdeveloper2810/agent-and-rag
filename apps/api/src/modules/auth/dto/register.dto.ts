import { z } from "zod";

export const registerSchema = z.object({
  email: z.string().email("Email không hợp lệ"),
  password: z.string().min(8, "Mật khẩu phải có ít nhất 8 ký tự"),
  name: z
    .string()
    .min(1, "Tên không được để trống")
    .max(100, "Tên không được vượt quá 100 ký tự"),
});

export type RegisterInput = z.infer<typeof registerSchema>;
