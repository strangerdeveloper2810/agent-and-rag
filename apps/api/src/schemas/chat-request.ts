import { z } from "zod";

/** Body cho POST /conversations/:id/chat — nội dung tin nhắn user + optional attachments. */
export const chatBodySchema = z.object({
  content: z.string().trim().min(1, "content không được rỗng"),
  attachments: z
    .array(
      z.object({
        type: z.enum(["image", "file"]),
        name: z.string(),
        data: z.string(),
        mimeType: z.string(),
      }),
    )
    .optional(),
  /**
   * Ngôn ngữ UI người dùng đang chọn ở FE (vd i18next `i18n.language`).
   * Optional — vắng mặt giữ hành vi mặc định (tiếng Việt). Forward nguyên văn
   * xuống agent-go qua goAgentClient để JARVIS trả lời đúng ngôn ngữ.
   */
  lang: z.enum(["vi", "en"]).optional(),
});

/** Body cho POST /conversations — tin nhắn đầu tiên (tùy chọn). */
export const createConversationBodySchema = z.object({
  firstMessage: z.string().optional(),
});

/**
 * Body cho POST /conversations/:id/resume — tiếp tục 1 run đã dừng giữa
 * chừng (interrupt HITL hoặc crash), xác định qua `runId` mà FE nhận được
 * từ event SSE `interrupt` (field `runId`, xem packages/types ChatEvent).
 * `answer` optional: chỉ cần khi run đang chờ trả lời 1 câu hỏi interrupt
 * thật — thiếu `answer` trong trường hợp đó sẽ bị agent-go trả lỗi 400.
 */
export const resumeBodySchema = z.object({
  runId: z.string().trim().min(1, "runId không được rỗng"),
  answer: z.string().optional(),
});
