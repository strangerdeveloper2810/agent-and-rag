import type { FastifyRequest } from "fastify";
import * as docService from "../services";
import { BadRequestError, NotFoundError } from "../../../lib/errors";

// Cùng pattern với modules/upload/upload.routes.ts — authGuard đã set req.tenantId,
// fallback "default" cho request không qua guard (không nên xảy ra, các route
// đều có authGuard, nhưng giữ fallback nhất quán với upload module).
const getTenantId = (req: FastifyRequest): string =>
  ((req as unknown as Record<string, unknown>).tenantId as string) ?? "default";

// Upload 1–7 file MỚI (.txt/.md/.pdf) → mỗi file thành 1 document version 1.
// Best-effort: file lỗi không làm hỏng file khác; trả kết quả từng file.
// (Cap 7 file: xem limits.files của multipart trong app.ts.)
export const uploadDocuments = async (req: FastifyRequest) => {
  const tenantId = getTenantId(req);
  const files: { filename: string; buffer: Buffer }[] = [];
  for await (const part of req.files()) {
    files.push({ filename: part.filename, buffer: await part.toBuffer() });
  }
  if (files.length === 0) throw new BadRequestError("Thiếu file");
  const results = await docService.ingestUploads(tenantId, files);
  return { results };
};

// Cập nhật tài liệu → version mới
export const updateDocument = async (req: FastifyRequest) => {
  const tenantId = getTenantId(req);
  const { documentId } = req.params as { documentId: string };
  const file = await req.file();
  if (!file) throw new BadRequestError("Thiếu file");
  const buffer = await file.toBuffer();
  return docService.updateUpload(tenantId, documentId, file.filename, buffer);
};

export const listDocuments = async (req: FastifyRequest) =>
  docService.listDocuments(getTenantId(req));

export const getVersions = async (req: FastifyRequest) => {
  const tenantId = getTenantId(req);
  const { documentId } = req.params as { documentId: string };
  return docService.getVersions(tenantId, documentId);
};

export const getVersionContent = async (req: FastifyRequest) => {
  const tenantId = getTenantId(req);
  const { documentId, version } = req.params as {
    documentId: string;
    version: string;
  };
  const result = await docService.getVersionContent(
    tenantId,
    documentId,
    Number(version),
  );
  if (!result.found) throw new NotFoundError("Không tìm thấy version");
  return result;
};

export const deleteDocument = async (req: FastifyRequest) => {
  const tenantId = getTenantId(req);
  const { documentId } = req.params as { documentId: string };
  return docService.removeDocument(tenantId, documentId);
};
