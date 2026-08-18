import { z } from "zod";

export const createUserSkillSchema = z.object({
  name: z
    .string()
    .min(1, "Tên skill không được để trống")
    .max(100, "Tên tối đa 100 ký tự")
    .regex(
      /^[a-zA-Z0-9_-]+$/,
      "Tên chỉ chứa chữ cái, số, dấu gạch ngang và gạch dưới",
    ),
  description: z.string().max(500, "Mô tả tối đa 500 ký tự").optional(),
  when_to_use: z
    .string()
    .max(1000, "Khi nào dùng tối đa 1000 ký tự")
    .optional(),
  content: z
    .string()
    .min(1, "Nội dung skill không được để trống")
    .max(10000, "Nội dung tối đa 10.000 ký tự"),
  triggers: z.array(z.string().max(100)).max(20).optional(),
});

export const updateUserSkillSchema = z.object({
  name: z
    .string()
    .min(1)
    .max(100)
    .regex(/^[a-zA-Z0-9_-]+$/)
    .optional(),
  description: z.string().max(500).optional(),
  when_to_use: z.string().max(1000).optional(),
  content: z.string().max(10000).optional(),
  triggers: z.array(z.string().max(100)).max(20).optional(),
  enabled: z.boolean().optional(),
});

export const toggleSkillSchema = z.object({
  enabled: z.boolean(),
});

export type CreateUserSkillInput = z.infer<typeof createUserSkillSchema>;
export type UpdateUserSkillInput = z.infer<typeof updateUserSkillSchema>;
export type ToggleSkillInput = z.infer<typeof toggleSkillSchema>;
