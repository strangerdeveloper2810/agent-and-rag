import type { FastifyInstance, preHandlerAsyncHookHandler } from "fastify";
import { documentsRoutes } from "./documents.routes";

// TODO(auth): Uncomment authGuard when jsonwebtoken is installed in Phase 3.
// import { authGuard } from '../../../common/guards/auth.guard';

const authGuard: preHandlerAsyncHookHandler = async (_req) => {
  // TODO(auth): Replace with real auth guard when Phase 3 is implemented
};

/**
 * Documents module — Fastify plugin gói gọn tất cả document routes
 * cùng với auth guard (hiện là placeholder chờ Phase 3).
 *
 * Đăng ký trong app.ts: `app.register(documentsModule, { prefix: "/api" });`
 */
export async function documentsModule(app: FastifyInstance): Promise<void> {
  app.addHook("onRequest", authGuard);
  await documentsRoutes(app);
}
