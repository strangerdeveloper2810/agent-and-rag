import type { FastifyInstance, FastifyReply } from "fastify";
import { ingestDocument, updateDocument } from "./documents.service";
import {
  listDocuments,
  deleteDocument,
  getVersions,
  getVersionContent,
} from "./documents.repository";
import {
  extractDocumentText,
  UnsupportedFileError,
  EmptyContentError,
} from "./extract";
import { VoyageError } from "../../lib/voyage";

// Map lỗi nạp tài liệu sang HTTP status + message thân thiện (toast hiển thị).
function handleUploadError(err: unknown, reply: FastifyReply) {
  if (err instanceof UnsupportedFileError) {
    return reply.code(415).send({ error: err.message });
  }
  if (err instanceof EmptyContentError) {
    return reply.code(422).send({ error: err.message });
  }
  if (err instanceof VoyageError && err.status === 429) {
    return reply.code(429).send({
      error:
        "Voyage đang giới hạn tốc độ (free tier: 3 request/phút, 10K token/phút). " +
        "Thêm payment method tại dashboard.voyageai.com để nâng giới hạn (vẫn miễn phí 200M token), " +
        "hoặc đợi ~1 phút rồi thử lại với file nhỏ hơn.",
    });
  }
  // File quá lớn (vượt limits.fileSize của multipart)
  if (
    err instanceof Error &&
    (err as { code?: string }).code === "FST_REQ_FILE_TOO_LARGE"
  ) {
    return reply.code(413).send({ error: "File quá lớn (tối đa 25MB)." });
  }
  throw err;
}

export async function documentsRoutes(app: FastifyInstance) {
  // Upload file .txt/.md/.pdf MỚI → trích text → chunk → embed → lưu (version 1)
  app.post("/documents/upload", async (req, reply) => {
    const file = await req.file();
    if (!file) return reply.code(400).send({ error: "Thiếu file" });
    try {
      const buffer = await file.toBuffer();
      const content = await extractDocumentText(file.filename, buffer);
      return await ingestDocument(file.filename, content);
    } catch (err) {
      return handleUploadError(err, reply);
    }
  });

  // CẬP NHẬT tài liệu đã có → tạo version mới (file mới có thể khác tên/định dạng)
  app.put("/documents/:documentId", async (req, reply) => {
    const { documentId } = req.params as { documentId: string };
    const file = await req.file();
    if (!file) return reply.code(400).send({ error: "Thiếu file" });
    try {
      const buffer = await file.toBuffer();
      const content = await extractDocumentText(file.filename, buffer);
      return await updateDocument(documentId, file.filename, content);
    } catch (err) {
      return handleUploadError(err, reply);
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
