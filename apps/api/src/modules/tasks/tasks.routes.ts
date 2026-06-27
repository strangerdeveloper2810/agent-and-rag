import type { FastifyInstance } from "fastify";
import { listTasks } from "./tasks.repository";

// Route debug: quan sát các task mà agent tạo qua tool (không phải user gọi trực tiếp)
export async function tasksRoutes(app: FastifyInstance) {
  app.get("/tasks", async () => listTasks({}));
}
