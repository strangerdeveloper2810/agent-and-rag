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
    GOOGLE_MODEL: z.string().default("gemini-3.1-flash-lite"),
    // Mức "thinking" của Gemini 3.x: OFF | LOW | MEDIUM | HIGH.
    // Gemini 3.x bật thinking mặc định → mỗi lượt tốn 30-60s dù output nhỏ.
    // LOW = nhanh nhất (đủ cho agent gọi tool). Đặt OFF nếu dùng model KHÔNG hỗ
    // trợ thinking (vd gemini-2.0-flash) để tránh lỗi.
    GOOGLE_THINKING_LEVEL: z
      .enum(["OFF", "LOW", "MEDIUM", "HIGH"])
      .default("LOW"),
    // Backend chạy agent: "langgraph" (in-process, hiện tại) | "go" (proxy sang
    // service agent-go — bật ở P12). Gateway chọn AgentClient theo giá trị này.
    AGENT_BACKEND: z.enum(["langgraph", "go"]).default("langgraph"),

    // ----- Vận hành -----
    NODE_ENV: z
      .enum(["development", "production", "test"])
      .default("development"),
    // Danh sách origin cho CORS, phân tách bằng dấu phẩy. Rỗng = cho mọi origin (dev).
    CORS_ORIGIN: z.string().optional(),
    // LangSmith tracing (tùy chọn) — LangChain đọc trực tiếp process.env; khai ở
    // đây để env là 1 NGUỒN SỰ THẬT (validate mềm, không bắt buộc).
    LANGSMITH_TRACING: z.string().optional(),
    LANGSMITH_PROJECT: z.string().optional(),
    LANGSMITH_API_KEY: z.string().optional(),
  })
  // Nếu chọn Google thì bắt buộc có GOOGLE_API_KEY.
  .refine((c) => c.LLM_PROVIDER !== "google" || !!c.GOOGLE_API_KEY, {
    message: "GOOGLE_API_KEY is required when LLM_PROVIDER=google",
    path: ["GOOGLE_API_KEY"],
  });

// Parse ngay lúc import → thiếu env là crash sớm với message rõ ràng
export const config = envSchema.parse(process.env);
export type Config = z.infer<typeof envSchema>;
