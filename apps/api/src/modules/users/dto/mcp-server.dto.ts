import { z } from "zod";

// transport: giao thức MCP server remote nói chuyện.
// - "http": Streamable HTTP (POST 1 endpoint, Accept: application/json,
//   text/event-stream) -- mặc định cho server mới, đúng spec MCP hiện tại.
// - "sse": giá trị legacy, giữ để không phá server đã cấu hình từ trước
//   migration 004 (về mặt wire protocol, agent-go dùng chung 1 client
//   Streamable HTTP cho cả 2 giá trị -- xem services/agent-go/internal/mcp/sse.go).
const transportSchema = z.enum(["http", "sse"]);

// auth_token: token xác thực gửi lên MCP server dưới dạng
// `Authorization: Bearer <token>` (hầu hết MCP server remote thật -- Notion,
// GitHub, Linear, Sentry... đều đòi header này). LƯU Ý: đây là secret -- API
// KHÔNG BAO GIỜ trả token này ra ngoài (xem users.controller.ts/toPublicMcpServer),
// chỉ trả `has_auth: boolean`.
const authTokenSchema = z.string().max(4096);

export const createMcpServerSchema = z.object({
  name: z
    .string()
    .min(1, "Tên MCP server không được để trống")
    .max(100, "Tên tối đa 100 ký tự")
    .regex(
      /^[a-zA-Z0-9_-]+$/,
      "Tên chỉ chứa chữ cái, số, dấu gạch ngang và gạch dưới",
    ),
  transport: transportSchema.default("http"),
  url: z.string().url("URL endpoint không hợp lệ"),
  auth_token: authTokenSchema.optional(),
});

export const updateMcpServerSchema = z.object({
  name: z
    .string()
    .min(1)
    .max(100)
    .regex(/^[a-zA-Z0-9_-]+$/)
    .optional(),
  transport: transportSchema.optional(),
  url: z.string().url("URL endpoint không hợp lệ").optional(),
  // auth_token: "" (chuỗi rỗng) nghĩa là XOÁ token hiện có (set NULL ở
  // repository). Không truyền field này (undefined) nghĩa là GIỮ NGUYÊN token
  // cũ -- xem users.repository.ts updateMcpServer.
  auth_token: authTokenSchema.optional(),
  enabled: z.boolean().optional(),
});

export type CreateMcpServerInput = z.infer<typeof createMcpServerSchema>;
export type UpdateMcpServerInput = z.infer<typeof updateMcpServerSchema>;
