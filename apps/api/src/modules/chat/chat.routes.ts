import type { FastifyInstance } from "fastify";
import * as ctrl from "./controllers";

// Routes = chỉ map đường dẫn → controller (thin), không chứa logic.
export const chatRoutes = async (app: FastifyInstance) => {
  app.post("/conversations", ctrl.postConversation);
  app.get("/conversations", ctrl.getConversations);
  app.get("/conversations/:id/messages", ctrl.getConversationMessages);
  app.delete("/conversations/:id", ctrl.deleteConversation);
  // Chat gọi LLM (tốn tiền) → siết chặt hơn mức toàn cục.
  app.post(
    "/conversations/:id/chat",
    { config: { rateLimit: { max: 20, timeWindow: "1 minute" } } },
    ctrl.postChat,
  );
};
