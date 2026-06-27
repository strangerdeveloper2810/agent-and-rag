import Fastify, { type FastifyInstance } from "fastify";
import cors from "@fastify/cors";
import multipart from "@fastify/multipart";
import { chatRoutes } from "./modules/chat/chat.routes";
import { documentsRoutes } from "./modules/documents/documents.routes";

export function buildApp(): FastifyInstance {
  const app = Fastify({ logger: true });

  app.register(cors, { origin: true });
  app.register(multipart);

  app.get("/api/health", async () => ({ status: "ok" }));

  // Các module:
  app.register(chatRoutes, { prefix: "/api" });
  app.register(documentsRoutes, { prefix: "/api" });
  // app.register(tasksRoutes, { prefix: "/api" });

  return app;
}

