import { z } from "zod";

export const messageRoleSchema = z.enum(["user", "assistant", "tool"]);

export type MessageRole = z.infer<typeof messageRoleSchema>;

export const messageSchema = z.object({
  conversationId: z.string(),
  role: messageRoleSchema,
  content: z.string(),
  toolCalls: z.array(z.record(z.unknown())).optional(),
  createdAt: z.date(),
});

export type Message = z.infer<typeof messageSchema>;
