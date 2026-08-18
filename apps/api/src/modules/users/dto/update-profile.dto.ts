import { z } from "zod";

export const updateProfileSchema = z.object({
  name: z.string().min(1, "Tên không được để trống").max(100).optional(),
  avatar_url: z.string().url("URL ảnh đại diện không hợp lệ").optional().nullable(),
});

export type UpdateProfileInput = z.infer<typeof updateProfileSchema>;
