import Fastify, { type FastifyInstance } from "fastify";
import cors from "@fastify/cors";
import multipart from "@fastify/multipart";
import { chatRoutes } from "./modules/chat/chat.routes";
import { documentsRoutes } from "./modules/documents/documents.routes";
import { tasksRoutes } from "./modules/tasks/tasks.routes";

export function buildApp(): FastifyInstance {
  const app = Fastify({ logger: true });

  app.register(cors, { origin: true });
  // PDF lớn hơn text nhiều → nới giới hạn file lên 25MB
  app.register(multipart, { limits: { fileSize: 25 * 1024 * 1024 } });

  app.get("/api/health", async () => ({ status: "ok" }));

  // Các module:
  app.register(chatRoutes, { prefix: "/api" });
  app.register(documentsRoutes, { prefix: "/api" });
  app.register(tasksRoutes, { prefix: "/api" });

  return app;
}

