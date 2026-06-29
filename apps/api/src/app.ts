import Fastify, { type FastifyInstance } from "fastify";
import cors from "@fastify/cors";
import multipart from "@fastify/multipart";
import { chatRoutes } from "./modules/chat";
import { documentsRoutes } from "./modules/documents";
import { tasksRoutes } from "./modules/tasks";
import { registerErrorHandler } from "./middleware/error-handler";

export function buildApp(): FastifyInstance {
  const app = Fastify({ logger: true });

  app.register(cors, { origin: true });
  // PDF lớn hơn text nhiều → nới giới hạn file lên 25MB
  app.register(multipart, { limits: { fileSize: 25 * 1024 * 1024 } });

  // Middleware: error handler tập trung (controller chỉ cần throw)
  registerErrorHandler(app);

  app.get("/api/health", async () => ({ status: "ok" }));

  // Các module:
  app.register(chatRoutes, { prefix: "/api" });
  app.register(documentsRoutes, { prefix: "/api" });
  app.register(tasksRoutes, { prefix: "/api" });

  return app;
}

