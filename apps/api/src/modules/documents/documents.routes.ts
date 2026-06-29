import type { FastifyInstance } from "fastify";
import { ingestDocument } from "./documents.service";
import { listSources, deleteSource } from "./documents.repository";
import { VoyageError } from "../../lib/voyage";

export async function documentsRoutes(app: FastifyInstance) {
  // Upload file .txt/.md → chunk → embed → lưu
  app.post("/documents/upload", async (req, reply) => {
    const file = await req.file();
    if (!file) return reply.code(400).send({ error: "Thiếu file" });
    const buffer = await file.toBuffer();
    const content = buffer.toString("utf-8");
    try {
      return await ingestDocument(file.filename, content);
    } catch (err) {
      if (err instanceof VoyageError && err.status === 429) {
        return reply.code(429).send({
          error:
            "Voyage đang giới hạn tốc độ (free tier: 3 request/phút, 10K token/phút). " +
            "Thêm payment method tại dashboard.voyageai.com để nâng giới hạn (vẫn miễn phí 200M token), " +
            "hoặc đợi ~1 phút rồi thử lại với file nhỏ hơn.",
        });
      }
      throw err;
    }
  });

  app.get("/documents", async () => listSources());

  app.delete("/documents/:source", async (req) => {
    const { source } = req.params as { source: string };
    await deleteSource(decodeURIComponent(source));
    return { ok: true };
  });
}
