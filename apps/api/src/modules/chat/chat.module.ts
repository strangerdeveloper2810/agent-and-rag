// TODO(auth): Uncomment authGuard when jsonwebtoken is installed in Phase 3.
// import { authGuard } from '../../common/guards/auth.guard';

import type { FastifyInstance, preHandlerAsyncHookHandler } from "fastify";
import { chatRoutes } from "./chat.routes";

/**
 * Placeholder auth guard — passes through all requests until Phase 3 auth
 * is implemented. Replace with the real import from
 * `../../common/guards/auth.guard` once jsonwebtoken is installed.
 */
const authGuard: preHandlerAsyncHookHandler = async (_req) => {
  // TODO(auth): Replace with real auth guard when Phase 3 is implemented.
};

/**
 * Chat module — Fastify plugin wrapping all chat routes with auth guard
 * and existing rate limits.
 *
 * Routes (prefixed with `/api` by the parent registration):
 * - POST   /api/conversations              — create new conversation
 * - GET    /api/conversations              — list all conversations
 * - GET    /api/conversations/:id/messages — get messages for a conversation
 * - DELETE /api/conversations/:id          — delete a conversation
 * - POST   /api/conversations/:id/chat     — chat with LLM (20 req/min)
 */
export const chatModule = async (app: FastifyInstance): Promise<void> => {
  // Apply auth guard to all chat routes in this scope.
  app.addHook("onRequest", authGuard);

  // Register the 5 chat routes (rate limits are defined per-route in
  // chat.routes.ts).
  await app.register(chatRoutes);
};
