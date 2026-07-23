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

export function buildApp(): FastifyInstance {
  // Tắt logger khi chạy test cho đỡ nhiễu output.
  const app = Fastify({ logger: config.NODE_ENV !== "test" });

  // CORS: whitelist theo CORS_ORIGIN (prod) hoặc mọi origin (dev khi rỗng).
  app.register(cors, {
    origin: config.CORS_ORIGIN
      ? config.CORS_ORIGIN.split(",").map((o) => o.trim())
      : true,
  });
  // PDF lớn hơn text nhiều → nới giới hạn file lên 25MB. Tối đa 7 file/lần upload.
  app.register(multipart, {
    limits: { fileSize: 25 * 1024 * 1024, files: 7 },
  });
  // Chặn abuse/DoS-chi-phí ở mức toàn cục. Endpoint tốn tiền (chat/upload) siết
  // chặt hơn qua config.rateLimit ở từng route.
  app.register(rateLimit, { max: 120, timeWindow: "1 minute" });

  // Middleware: error handler tập trung (controller chỉ cần throw)
  registerErrorHandler(app);

  // Liveness: process còn sống (không phụ thuộc DB).
  app.get("/api/health", async () => ({ status: "ok" }));

  // Readiness: sẵn sàng phục vụ (ping được Mongo). Chưa sẵn sàng → 503.
  app.get("/api/ready", async (_req, reply) => {
    try {
      await getDb().command({ ping: 1 });
      return { status: "ready" };
    } catch {
      return reply.code(503).send({ status: "not_ready" });
    }
  });

  // Các module:
  app.register(chatRoutes, { prefix: "/api" });
  app.register(documentsRoutes, { prefix: "/api" });
  app.register(tasksRoutes, { prefix: "/api" });

  return app;
}
