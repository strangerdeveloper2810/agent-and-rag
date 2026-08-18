import { z } from "zod";

export const updateProfileSchema = z.object({
  name: z.string().min(1, "Tên không được để trống").max(100).optional(),
  avatar_url: z
    .string()
    .url("URL ảnh đại diện không hợp lệ")
    .optional()
    .nullable(),
});

export const avatarUrlSchema = z.object({
  avatar_url: z.string().url("URL ảnh đại diện không hợp lệ").nullable(),
});

export type UpdateProfileInput = z.infer<typeof updateProfileSchema>;
export type AvatarUrlInput = z.infer<typeof avatarUrlSchema>;
