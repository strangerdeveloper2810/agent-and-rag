import type { FastifyInstance } from "fastify";
import * as ctrl from "./controllers";
import { authGuard } from "../../common/guards/auth.guard";

export async function tasksRoutes(app: FastifyInstance) {
  app.get("/tasks", { preHandler: [authGuard] }, ctrl.getTasks);
}
