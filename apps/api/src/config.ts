import { z } from "zod";

const envSchema = z
  .object({
    // ----- Server -----
    PORT: z.coerce.number().default(3001),
    NODE_ENV: z
      .enum(["development", "production", "test"])
      .default("development"),

    // ----- Database -----
    MONGODB_URI: z.string().min(1, "MONGODB_URI is required"),

    // ----- AI Provider: Anthropic -----
    ANTHROPIC_API_KEY: z.string().min(1, "ANTHROPIC_API_KEY is required"),
    CLAUDE_MODEL: z.string().default("claude-haiku-4-5-20251001"),

    // ----- AI Provider: Google (optional) -----
    LLM_PROVIDER: z.enum(["anthropic", "google"]).default("anthropic"),
    GOOGLE_API_KEY: z.string().optional(),
    GOOGLE_MODEL: z.string().default("gemini-3.1-flash-lite"),
    // Mức "thinking" của Gemini 3.x: OFF | LOW | MEDIUM | HIGH.
    // LOW = nhanh nhất (đủ cho agent gọi tool).
    GOOGLE_THINKING_LEVEL: z
      .enum(["OFF", "LOW", "MEDIUM", "HIGH"])
      .default("LOW"),

    // ----- Embedding -----
    VOYAGE_API_KEY: z.string().min(1, "VOYAGE_API_KEY is required"),

    // ----- Agent Backend -----
    // "langgraph" = in-process LangChain (legacy).
    // "go" = proxy HTTP+SSE sang service agent-go.
    AGENT_BACKEND: z.enum(["langgraph", "go"]).default("langgraph"),
    /** URL gốc của Go agent runtime (dùng khi AGENT_BACKEND=go). */
    AGENT_GO_URL: z.string().default("http://localhost:3002"),
    /** Timeout (ms) cho mỗi HTTP request sang Go agent (chat + health check). */
    AGENT_GO_TIMEOUT: z.coerce.number().int().positive().default(120_000),

    // ----- Vận hành -----
    /** Danh sách origin cho CORS, phân tách bằng dấu phẩy. Rỗng = cho mọi origin (dev). */
    CORS_ORIGIN: z.string().optional(),

    // ----- LangSmith tracing (tùy chọn) -----
    LANGSMITH_TRACING: z.string().optional(),
    LANGSMITH_PROJECT: z.string().optional(),
    LANGSMITH_API_KEY: z.string().optional(),
  })
  // Nếu chọn Google thì bắt buộc có GOOGLE_API_KEY.
  .refine((c) => c.LLM_PROVIDER !== "google" || !!c.GOOGLE_API_KEY, {
    message: "GOOGLE_API_KEY is required when LLM_PROVIDER=google",
    path: ["GOOGLE_API_KEY"],
  });

/** Cấu hình ứng dụng được parse+validate từ env lúc import. */
export const config = envSchema.parse(process.env);

/** Kiểu type-safe của toàn bộ config (dùng `z.infer` từ zod schema). */
export type Config = z.infer<typeof envSchema>;
