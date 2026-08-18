/**
 * Danh mục queryKey — MỘT nguồn sự thật cho mọi key của TanStack Query.
 *
 * Lý do phải tập trung: staleTime và dedupe hoạt động theo key. Hai chỗ gõ
 * tay ["session"] và ["Session"] là hai cache khác nhau → lại gọi API trùng,
 * đúng cái bug đang đi sửa. Ngoài ra invalidate theo prefix chỉ đúng khi key
 * được xếp tầng nhất quán (vd ["user", ...] để xoá sạch dữ liệu user một lần).
 */

export const queryKeys = {
  /** GET /api/auth/me — user của session hiện tại (null nếu chưa đăng nhập). */
  session: () => ["session"] as const,

  /** Prefix chung cho dữ liệu thuộc về user — dùng để invalidate cả nhóm. */
  user: () => ["user"] as const,

  /** GET /api/user/settings */
  settings: () => ["user", "settings"] as const,

  /** GET /api/user/mcp-servers */
  mcpServers: () => ["user", "mcp-servers"] as const,

  /** GET /api/user/skills */
  skills: () => ["user", "skills"] as const,

  /** GET /api/conversations */
  conversations: () => ["conversations"] as const,

  /** GET /api/conversations/:id/messages */
  messages: (conversationId: string) =>
    ["conversations", conversationId, "messages"] as const,

  /** GET {agent}/suggestions — LƯU Ý: một lượt gọi LLM, xem STALE_TIME. */
  suggestions: () => ["suggestions"] as const,
} as const;
