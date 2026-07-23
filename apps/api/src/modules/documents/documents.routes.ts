import type { FastifyInstance } from "fastify";
import * as ctrl from "./controllers";

// Routes = chỉ map đường dẫn → controller (thin). Lỗi do error handler tập trung lo.
export async function documentsRoutes(app: FastifyInstance) {
  // Upload gọi Voyage embedding (tốn tiền, free tier 3 RPM) → siết chặt hơn.
  const embedLimit = {
    config: { rateLimit: { max: 20, timeWindow: "1 minute" } },
  };
  app.post("/documents/upload", embedLimit, ctrl.uploadDocument);
  app.put("/documents/:documentId", embedLimit, ctrl.updateDocument);
  app.get("/documents", ctrl.listDocuments);
  app.get("/documents/:documentId/versions", ctrl.getVersions);
  app.get("/documents/:documentId/versions/:version", ctrl.getVersionContent);
  app.delete("/documents/:documentId", ctrl.deleteDocument);
}
