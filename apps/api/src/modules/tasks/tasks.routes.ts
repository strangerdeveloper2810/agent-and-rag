import type { FastifyInstance } from "fastify";
import * as ctrl from "./controllers";

export async function tasksRoutes(app: FastifyInstance) {
  app.get("/tasks", ctrl.getTasks);
}
