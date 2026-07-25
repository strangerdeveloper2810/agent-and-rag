// TODO(auth): Uncomment authGuard when jsonwebtoken is installed in Phase 3.
// import { authGuard } from '../../common/guards/auth.guard';

import type { FastifyInstance, preHandlerAsyncHookHandler } from "fastify";
import { tasksRoutes } from "./tasks.routes";

/**
 * Placeholder auth guard — passes through all requests until Phase 3 auth
 * is implemented. Replace with the real import from
 * `../../common/guards/auth.guard` once jsonwebtoken is installed.
 */
const authGuard: preHandlerAsyncHookHandler = async (_req) => {
  // TODO(auth): Replace with real auth guard when Phase 3 is implemented.
};

/**
 * Tasks module — Fastify plugin wrapping all tasks routes with auth guard.
 *
 * Routes (prefixed with `/api` by the parent registration):
 * - GET /api/tasks — list all tasks
 */
export const tasksModule = async (app: FastifyInstance): Promise<void> => {
  // Apply auth guard to all tasks routes in this scope.
  app.addHook("onRequest", authGuard);

  // Register the tasks routes.
  await app.register(tasksRoutes);
};
