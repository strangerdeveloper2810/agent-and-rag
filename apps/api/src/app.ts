import Fastify, { type FastifyInstance } from "fastify";
import cors from "@fastify/cors";

export function buildApp(): FastifyInstance {
  const app = Fastify({ logger: true });

  app.register(cors, { origin: true });

  app.get("/api/health", async () => ({ status: "ok" }));

  // Các module sẽ được register ở các mốc sau:
  // app.register(chatRoutes, { prefix: "/api" });
  // app.register(documentsRoutes, { prefix: "/api" });
  // app.register(tasksRoutes, { prefix: "/api" });

  return app;
}
