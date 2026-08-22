import type { AgentEvent } from "../graph/index";

// ----- AgentMessage & AgentStreamOptions -----

/** Một message trong lịch sử hội thoại gửi cho agent. */
export type AgentMessage = { role: string; content: string };

/** Tuỳ chọn cho agent stream (abort signal từ HTTP request). */
export type AgentStreamOptions = {
  signal?: AbortSignal;
  attachments?: Array<{
    type: string;
    name: string;
    data: string;
    mimeType: string;
  }>;
  /** req.tenantId (từ authGuard) — forward sang agent-go qua header X-Tenant-ID. */
  tenantId?: string;
  /**
   * Ngôn ngữ UI người dùng đang chọn ở FE ("vi" | "en"). Optional — không
   * truyền/undefined giữ hành vi mặc định (tiếng Việt). Chỉ goAgentClient
   * forward field này sang agent-go (body JSON `lang`); agent-go dùng nó để
   * ghi đè chỉ dẫn ngôn ngữ trong system prompt cho riêng lượt chạy này.
   */
  lang?: "vi" | "en";
  personaPreset?: string;
  formality?: string;
  verbosity?: string;
  customInstructions?: string;
  /** MCP servers (remote) do user cấu hình — forward sang agent-go để
   * discovery tools. transport: "http" (Streamable HTTP, mặc định) | "sse"
   * (legacy) — xem services/agent-go/internal/mcp/sse.go, cùng 1 client xử lý
   * cả 2 giá trị. apiKey mang giá trị auth_token đã lưu (đổi tên field JSON để
   * khớp Go McpServerInput.APIKey, xem chat.controller.ts). */
  mcpServers?: Array<{
    name: string;
    url: string;
    apiKey?: string;
    transport?: "http" | "sse";
  }>;
  /** Tên các builtin skill user đã tắt. */
  disabledSkills?: string[];
  /** Custom skills do user định nghĩa (prompt instruction text). */
  customSkills?: Array<{
    name: string;
    description?: string;
    whenToUse?: string;
    content?: string;
    triggers?: string[];
  }>;
};

/** Tuỳ chọn cho AgentClient.resume — nhẹ hơn AgentStreamOptions vì resume
 * không gửi lại history/attachments/persona (agent-go tự phục hồi từ
 * checkpoint đã lưu, xem services/agent-go/internal/agent/resume.go). */
export type AgentResumeOptions = {
  signal?: AbortSignal;
  /** req.tenantId (từ authGuard) — forward sang agent-go qua header X-Tenant-ID. */
  tenantId?: string;
};

/**
 * AgentClient — BIÊN GIỚI giữa gateway (Fastify) và agent runtime.
 */
export interface AgentClient {
  stream(
    history: AgentMessage[],
    opts?: AgentStreamOptions,
  ): AsyncIterable<AgentEvent>;

  /**
   * Tiếp tục 1 run đã dừng giữa chừng (interrupt HITL hoặc crash) theo
   * `runId` — xem services/agent-go/internal/transport/http/chat_resume.go.
   * `answer` optional: chỉ cần khi run đang chờ trả lời 1 câu hỏi interrupt
   * thật; bỏ trống nếu chỉ đơn giản resume từ node đã dừng.
   *
   * OPTIONAL vì chỉ agent-go (checkpoint/resume) hỗ trợ — LangGraph (legacy,
   * `AGENT_BACKEND=langgraph`) không có khái niệm checkpoint nên không
   * implement. Caller (chat.service.resumeReply) PHẢI kiểm tra tồn tại
   * trước khi gọi, ném lỗi rõ ràng nếu backend không hỗ trợ.
   */
  resume?(
    runId: string,
    answer: string | undefined,
    opts?: AgentResumeOptions,
  ): AsyncIterable<AgentEvent>;
}
