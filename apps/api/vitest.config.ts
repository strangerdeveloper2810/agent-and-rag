import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    globals: true,
    environment: "node",
    // Env giả cho unit test: chỉ cần để config.ts parse qua được,
    // KHÔNG dùng secret thật (test không gọi DB/LLM thật).
    env: {
      MONGODB_URI: "mongodb://localhost:27017/ai_agent_tut_test",
      ANTHROPIC_API_KEY: "test-anthropic-key",
      VOYAGE_API_KEY: "test-voyage-key",
    },
  },
});
