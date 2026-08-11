import type { FastifyInstance } from "fastify";
import * as ctrl from "./controllers";
import { authGuard } from "../../common/guards/auth.guard";

// Routes = chỉ map đường dẫn → controller (thin), không chứa logic.
export const chatRoutes = async (app: FastifyInstance) => {
  app.post(
    "/conversations",
    { preHandler: [authGuard] },
    ctrl.postConversation,
  );
  app.get("/conversations", { preHandler: [authGuard] }, ctrl.getConversations);
  app.get(
    "/conversations/:id/messages",
    { preHandler: [authGuard] },
    ctrl.getConversationMessages,
  );
  app.delete(
    "/conversations/:id",
    { preHandler: [authGuard] },
    ctrl.deleteConversation,
  );
  // Chat gọi LLM (tốn tiền) → siết chặt hơn mức toàn cục.
  app.post(
    "/conversations/:id/chat",
    {
      preHandler: [authGuard],
      config: { rateLimit: { max: 20, timeWindow: "1 minute" } },
    },
    ctrl.postChat,
  );
};
