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
});

/** Body cho POST /conversations — tin nhắn đầu tiên (tùy chọn). */
export const createConversationBodySchema = z.object({
  firstMessage: z.string().optional(),
});
