import { z } from "zod";

export const changePasswordSchema = z.object({
  oldPassword: z.string().min(1, "Mật khẩu cũ không được để trống"),
  newPassword: z.string().min(8, "Mật khẩu mới phải từ 8 ký tự trở lên"),
});

export type ChangePasswordInput = z.infer<typeof changePasswordSchema>;
