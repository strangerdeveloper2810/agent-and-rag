import type { FastifyInstance } from "fastify";
import * as ctrl from "./controllers";

// Routes = chỉ map đường dẫn → controller (thin). Lỗi do error handler tập trung lo.
export async function documentsRoutes(app: FastifyInstance) {
  app.post("/documents/upload", ctrl.uploadDocument);
  app.put("/documents/:documentId", ctrl.updateDocument);
  app.get("/documents", ctrl.listDocuments);
  app.get("/documents/:documentId/versions", ctrl.getVersions);
  app.get("/documents/:documentId/versions/:version", ctrl.getVersionContent);
  app.delete("/documents/:documentId", ctrl.deleteDocument);
}
