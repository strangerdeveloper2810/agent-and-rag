import { z } from "zod";

const envSchema = z.object({
  PORT: z.coerce.number().default(3001),
  MONGODB_URI: z.string().min(1, "MONGODB_URI is required"),
  ANTHROPIC_API_KEY: z.string().min(1, "ANTHROPIC_API_KEY is required"),
  VOYAGE_API_KEY: z.string().min(1, "VOYAGE_API_KEY is required"),
  CLAUDE_MODEL: z.string().default("claude-haiku-4-5-20251001"),
});

// Parse ngay lúc import → thiếu env là crash sớm với message rõ ràng
export const config = envSchema.parse(process.env);
export type Config = z.infer<typeof envSchema>;
