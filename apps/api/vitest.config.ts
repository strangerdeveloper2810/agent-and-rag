import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    globals: true,
    environment: "node",
    // Env giả cho unit test: chỉ cần để config.ts parse qua được,
    // KHÔNG dùng secret thật (test không gọi DB/LLM thật).
    env: {
      MONGODB_URI: "mongodb://localhost:27017/ai_agent_tut_test",
      PG_CONNECTION_STRING: "postgres://test:test@localhost:5432/test",
      REDIS_URL: "redis://localhost:6379",
      ANTHROPIC_API_KEY: "test-anthropic-key",
      VOYAGE_API_KEY: "test-voyage-key",
      JWT_SECRET: "test-jwt-secret-at-least-32-characters-long",
      JWT_REFRESH_SECRET: "test-jwt-refresh-secret-at-least-32-chars-long",
      GOOGLE_CLIENT_ID: "test-google-client-id",
      GOOGLE_CLIENT_SECRET: "test-google-client-secret",
      GOOGLE_REDIRECT_URI: "http://localhost:3001/api/auth/google/callback",
    },

    coverage: {
      provider: "v8",
      reporter: ["text", "json-summary"],

      // KHÔNG đặt ngưỡng toàn repo: apps/api hiện ~37% statements (server.ts,
      // email service, src/agent/deprecated/... chưa có test), nên một ngưỡng
      // global 90% chỉ làm CI đỏ vĩnh viễn chứ không ai sửa nổi trong một PR.
      //
      // Thay vào đó khoá ngưỡng theo TỪNG FILE đã có test tử tế. File nào được
      // viết test mới thì thêm vào đây — coverage chỉ đi lên, không tụt lại.
      thresholds: {
        "src/app.ts": {
          statements: 90,
          functions: 90,
          lines: 90,
          branches: 80,
        },
        "src/common/guards/rate-limit-key.ts": {
          statements: 100,
          functions: 100,
          lines: 100,
          branches: 100,
        },
      },
    },
  },
});
