import Fastify, { type FastifyInstance } from "fastify";
import cors from "@fastify/cors";
import multipart from "@fastify/multipart";
import rateLimit from "@fastify/rate-limit";
import { chatRoutes } from "./modules/chat";
import { documentsRoutes } from "./modules/documents";
import { tasksRoutes } from "./modules/tasks";
import { registerErrorHandler } from "./middleware/error-handler";
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

  // Chặn abuse/DoS ở mức toàn cục.
  app.register(rateLimit, { max: 120, timeWindow: "1 minute" });

  // Middleware: error handler tập trung (controller chỉ cần throw).
  registerErrorHandler(app);

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

  // ---- Feature modules (hiện tại dùng routes trực tiếp) ----
  //
  // Sau khi hoàn tất auth Phase 3, chuyển sang dùng module plugin:
  //   app.register(chatModule, { prefix: "/api" });
  //   app.register(documentsModule, { prefix: "/api" });
  //   app.register(tasksModule, { prefix: "/api" });
  //
  // Các `*Module` plugin đã được tạo sẵn trong `*.module.ts` với
  // authGuard placeholder. Khi `jsonwebtoken` được cài, chỉ cần
  // uncomment import authGuard + đổi tên plugin là xong.

  app.register(chatRoutes, { prefix: "/api" });
  app.register(documentsRoutes, { prefix: "/api" });
  app.register(tasksRoutes, { prefix: "/api" });

  return app;
}
