import type { FastifyInstance, FastifyReply } from "fastify";
import { ingestDocument, updateDocument } from "./documents.service";
import {
  listDocuments,
  deleteDocument,
  getVersions,
  getVersionContent,
} from "./documents.repository";
import { VoyageError } from "../../lib/voyage";

// Lỗi rate limit của Voyage (429) → trả message hướng dẫn thay vì 500
function handleVoyageError(err: unknown, reply: FastifyReply) {
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

export async function documentsRoutes(app: FastifyInstance) {
  // Upload file .txt/.md MỚI → chunk → embed → lưu (version 1)
  app.post("/documents/upload", async (req, reply) => {
    const file = await req.file();
    if (!file) return reply.code(400).send({ error: "Thiếu file" });
    const content = (await file.toBuffer()).toString("utf-8");
    try {
      return await ingestDocument(file.filename, content);
    } catch (err) {
      return handleVoyageError(err, reply);
    }
  });

  // CẬP NHẬT tài liệu đã có → tạo version mới (file mới có thể khác tên)
  app.put("/documents/:documentId", async (req, reply) => {
    const { documentId } = req.params as { documentId: string };
    const file = await req.file();
    if (!file) return reply.code(400).send({ error: "Thiếu file" });
    const content = (await file.toBuffer()).toString("utf-8");
    try {
      return await updateDocument(documentId, file.filename, content);
    } catch (err) {
      return handleVoyageError(err, reply);
    }
  });

  app.get("/documents", async () => listDocuments());

  // Lịch sử version của 1 tài liệu
  app.get("/documents/:documentId/versions", async (req) => {
    const { documentId } = req.params as { documentId: string };
    return getVersions(documentId);
  });

  // Nội dung 1 version cụ thể (để xem lại bản cũ)
  app.get("/documents/:documentId/versions/:version", async (req, reply) => {
    const { documentId, version } = req.params as {
      documentId: string;
      version: string;
    };
    const result = await getVersionContent(documentId, Number(version));
    if (!result.found) return reply.code(404).send({ error: "Không tìm thấy version" });
    return result;
  });

  app.delete("/documents/:documentId", async (req) => {
    const { documentId } = req.params as { documentId: string };
    return deleteDocument(documentId);
  });
}
