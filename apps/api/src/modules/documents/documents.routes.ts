import type { FastifyInstance } from "fastify";
import { ingestDocument } from "./documents.service";
import { listSources, deleteSource } from "./documents.repository";

export async function documentsRoutes(app: FastifyInstance) {
  // Upload file .txt/.md → chunk → embed → lưu
  app.post("/documents/upload", async (req, reply) => {
    const file = await req.file();
    if (!file) return reply.code(400).send({ error: "Thiếu file" });
    const buffer = await file.toBuffer();
    const content = buffer.toString("utf-8");
    return ingestDocument(file.filename, content);
  });

  app.get("/documents", async () => listSources());

  app.delete("/documents/:source", async (req) => {
    const { source } = req.params as { source: string };
    await deleteSource(decodeURIComponent(source));
    return { ok: true };
  });
}
