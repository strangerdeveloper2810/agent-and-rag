import Fastify, { type FastifyInstance } from "fastify";
import cors from "@fastify/cors";
import cookie from "@fastify/cookie";
import multipart from "@fastify/multipart";
import rateLimit from "@fastify/rate-limit";
import { chatRoutes } from "./modules/chat";
import { documentsRoutes } from "./modules/documents";
import { tasksRoutes } from "./modules/tasks";
import { uploadModule } from "./modules/upload/upload.module";
import { authModule } from "./modules/auth/auth.module";
import { usersModule } from "./modules/users/users.module";
import { getPgPool } from "./database/index.js";
import { registerErrorFilter } from "./common/filters/error.filter";
import { rateLimitKeyGenerator } from "./common/guards/rate-limit-key";
import { getDb } from "./lib/mongo";
import { config } from "./config";
import { checkGoAgentHealth } from "./agent/client";

/**
 * Build và cấu hình Fastify instance.
 *
 * Các endpoint sức khoẻ:
 * - `/api/health`   — Liveness: process còn sống (không phụ thuộc DB/service ngoài).
 * - `/api/healthz`  — Deep health: MongoDB + Go agent (nếu dùng backend=go).
 * - `/api/ready`    — Readiness: sẵn sàng phục vụ (ping được MongoDB).
 *
 * Tổ chức module:
 * - `app.ts` compose tất cả plugin/module vào 1 Fastify instance.
 * - Mỗi module (`chatModule`, `documentsModule`, `tasksModule`) là 1 Fastify
 *   plugin đóng gói route + auth guard + business logic của domain đó.
 * - Auth guard hiện là placeholder (pass-through) — sẽ được kích hoạt khi
 *   cài `jsonwebtoken` ở Phase 3 (xem TODO trong từng `*.module.ts`).
 */
export function buildApp(): FastifyInstance {
  // Tắt logger khi chạy test cho đỡ nhiễu output.
  const app = Fastify({
    logger: config.NODE_ENV !== "test",
    bodyLimit: 50 * 1024 * 1024, // 50MB — attachments + long conversation history
    // Số HOP proxy tin được, tính từ phía server: web(nginx container) → api là
    // 1, reverse proxy ngoài cùng là 2. Fastify sẽ bỏ qua 2 entry cuối của
    // X-Forwarded-For và lấy entry trước đó làm req.ip.
    //
    // Cố tình KHÔNG dùng `trustProxy: true`: khi tin tất cả các hop, Fastify lấy
    // entry ngoài cùng bên trái của X-Forwarded-For — mà client tự gửi được
    // header đó, nên bất kỳ ai cũng bơm được IP giả để reset hạn mức rate limit.
    // Đếm hop thì con số lấy ra luôn là IP mà proxy đầu tiên THỰC SỰ thấy.
    trustProxy: 2,
  });

  // ---- Cross-cutting plugins ----

  // CORS: whitelist theo CORS_ORIGIN (prod) hoặc mọi origin (dev khi rỗng).
  app.register(cors, {
    origin: config.CORS_ORIGIN
      ? config.CORS_ORIGIN.split(",").map((o) => o.trim())
      : true,
  });

  // PDF lớn hơn text nhiều -> nới giới hạn file lên 25MB. Tối đa 7 file/lần upload.
  app.register(multipart, {
    limits: { fileSize: 25 * 1024 * 1024, files: 7 },
  });

  // Cookie parser — cần cho auth guard đọc access_token/refresh_token.
  app.register(cookie);

  // Rate limiting chống abuse/DoS — in-memory (đủ cho single instance).
  // keyGenerator: khoá theo user đã đăng nhập, chỉ rơi về IP khi chưa đăng nhập
  // (xem rate-limit-key.ts — trước đây mọi user dùng chung một bucket).
  app.register(rateLimit, {
    max: 120,
    timeWindow: "1 minute",
    keyGenerator: rateLimitKeyGenerator,
  });

  // Error filter tập trung (thay thế middleware/error-handler.ts cũ).
  registerErrorFilter(app);

  // ---- Health endpoints ----

  /** Liveness: process còn sống (không phụ thuộc bất cứ service ngoài nào). */
  app.get("/api/health", async () => ({ status: "ok" }));

  /**
   * Deep health: kiểm tra MongoDB + Go agent (nếu AGENT_BACKEND=go).
   * Trả về trạng thái từng thành phần để monitoring có thể phân biệt
   * lỗi DB vs lỗi agent.
   */
  app.get("/api/healthz", async (_req, reply) => {
    const checks: Record<string, "ok" | "error"> = {};

    // MongoDB
    try {
      await getDb().command({ ping: 1 });
      checks.mongo = "ok";
    } catch {
      checks.mongo = "error";
    }

    // Go agent (chỉ check khi dùng backend=go)
    if (config.AGENT_BACKEND === "go") {
      const goHealthy = await checkGoAgentHealth();
      checks.go_agent = goHealthy ? "ok" : "error";
    }

    const allOk = Object.values(checks).every((v) => v === "ok");
    return reply.code(allOk ? 200 : 503).send({
      status: allOk ? "healthy" : "degraded",
      checks,
    });
  });

  /** Readiness: sẵn sàng phục vụ (ping được Mongo). Chưa sẵn sàng -> 503. */
  app.get("/api/ready", async (_req, reply) => {
    try {
      await getDb().command({ ping: 1 });
      return { status: "ready" };
    } catch {
      return reply.code(503).send({ status: "not_ready" });
    }
  });

  // ---- Auth + Admin modules ----
  app.register(authModule, { pgPool: getPgPool() });
  app.register(usersModule, { pgPool: getPgPool() });

  // ---- Feature modules ----
  app.register(chatRoutes, { prefix: "/api" });
  app.register(documentsRoutes, { prefix: "/api" });
  app.register(tasksRoutes, { prefix: "/api" });
  app.register(uploadModule);

  return app;
}
