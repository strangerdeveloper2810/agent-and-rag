import type { FastifyInstance } from "fastify";
import * as ctrl from "./controllers";
import { authGuard } from "../../common/guards/auth.guard";

// Routes = chỉ map đường dẫn → controller (thin). Lỗi do error handler tập trung lo.
export async function documentsRoutes(app: FastifyInstance) {
  // Upload gọi Voyage embedding (tốn tiền, free tier 3 RPM) → siết chặt hơn.
  const embedLimit = {
    preHandler: [authGuard],
    config: { rateLimit: { max: 20, timeWindow: "1 minute" } },
  };
  app.post("/documents/upload", embedLimit, ctrl.uploadDocuments);
  app.put("/documents/:documentId", embedLimit, ctrl.updateDocument);
  app.get("/documents", { preHandler: [authGuard] }, ctrl.listDocuments);
  app.get("/documents/:documentId/versions", { preHandler: [authGuard] }, ctrl.getVersions);
  app.get("/documents/:documentId/versions/:version", { preHandler: [authGuard] }, ctrl.getVersionContent);
  app.delete("/documents/:documentId", { preHandler: [authGuard] }, ctrl.deleteDocument);
}
