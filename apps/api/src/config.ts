import { z } from "zod";

const envSchema = z
  .object({
    PORT: z.coerce.number().default(3001),
    MONGODB_URI: z.string().min(1, "MONGODB_URI is required"),
    ANTHROPIC_API_KEY: z.string().min(1, "ANTHROPIC_API_KEY is required"),
    VOYAGE_API_KEY: z.string().min(1, "VOYAGE_API_KEY is required"),
    CLAUDE_MODEL: z.string().default("claude-haiku-4-5-20251001"),
    // Chọn nhà cung cấp model cho agent. Mặc định anthropic (giữ nguyên hành vi cũ).
    LLM_PROVIDER: z.enum(["anthropic", "google"]).default("anthropic"),
    GOOGLE_API_KEY: z.string().optional(),
    GOOGLE_MODEL: z.string().default("gemini-2.0-flash"),
  })
  // Nếu chọn Google thì bắt buộc có GOOGLE_API_KEY.
  .refine((c) => c.LLM_PROVIDER !== "google" || !!c.GOOGLE_API_KEY, {
    message: "GOOGLE_API_KEY is required when LLM_PROVIDER=google",
    path: ["GOOGLE_API_KEY"],
  });

// Parse ngay lúc import → thiếu env là crash sớm với message rõ ràng
export const config = envSchema.parse(process.env);
export type Config = z.infer<typeof envSchema>;
