import { z } from "zod";

export const createMcpServerSchema = z.object({
  name: z
    .string()
    .min(1, "Tên MCP server không được để trống")
    .max(100, "Tên tối đa 100 ký tự")
    .regex(
      /^[a-zA-Z0-9_-]+$/,
      "Tên chỉ chứa chữ cái, số, dấu gạch ngang và gạch dưới",
    ),
  url: z.string().url("URL endpoint không hợp lệ"),
  api_key: z.string().max(500).optional().nullable(),
});

export const updateMcpServerSchema = z.object({
  name: z
    .string()
    .min(1)
    .max(100)
    .regex(/^[a-zA-Z0-9_-]+$/)
    .optional(),
  url: z.string().url("URL endpoint không hợp lệ").optional(),
  api_key: z.string().max(500).optional().nullable(),
  enabled: z.boolean().optional(),
});

export type CreateMcpServerInput = z.infer<typeof createMcpServerSchema>;
export type UpdateMcpServerInput = z.infer<typeof updateMcpServerSchema>;
