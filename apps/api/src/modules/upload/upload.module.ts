import type { FastifyInstance, preHandlerAsyncHookHandler } from "fastify";
import { uploadRoutes } from "./upload.routes";

const authGuard: preHandlerAsyncHookHandler = async (_req) => {
  // TODO(auth): Replace with real auth guard when Phase 3 is implemented.
};

export const uploadModule = async (app: FastifyInstance): Promise<void> => {
  app.addHook("onRequest", authGuard);
  await uploadRoutes(app);
};
