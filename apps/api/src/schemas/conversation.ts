import z from "zod";

export const conversationSchema = z.object({
  title: z.string(),
  createdAt: z.date(),
  updatedAt: z.date(),
});

export type Conversation = z.infer<typeof conversationSchema>;
