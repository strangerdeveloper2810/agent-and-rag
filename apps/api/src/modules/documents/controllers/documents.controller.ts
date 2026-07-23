import type { FastifyRequest } from "fastify";
import * as docService from "../services";
import { BadRequestError, NotFoundError } from "../../../lib/errors";

// Upload 1–7 file MỚI (.txt/.md/.pdf) → mỗi file thành 1 document version 1.
// Best-effort: file lỗi không làm hỏng file khác; trả kết quả từng file.
// (Cap 7 file: xem limits.files của multipart trong app.ts.)
export const uploadDocuments = async (req: FastifyRequest) => {
  const files: { filename: string; buffer: Buffer }[] = [];
  for await (const part of req.files()) {
    files.push({ filename: part.filename, buffer: await part.toBuffer() });
  }
  if (files.length === 0) throw new BadRequestError("Thiếu file");
  const results = await docService.ingestUploads(files);
  return { results };
};

// Cập nhật tài liệu → version mới
export const updateDocument = async (req: FastifyRequest) => {
  const { documentId } = req.params as { documentId: string };
  const file = await req.file();
  if (!file) throw new BadRequestError("Thiếu file");
  const buffer = await file.toBuffer();
  return docService.updateUpload(documentId, file.filename, buffer);
};

export const listDocuments = async () => docService.listDocuments();

export const getVersions = async (req: FastifyRequest) => {
  const { documentId } = req.params as { documentId: string };
  return docService.getVersions(documentId);
};

export const getVersionContent = async (req: FastifyRequest) => {
  const { documentId, version } = req.params as {
    documentId: string;
    version: string;
  };
  const result = await docService.getVersionContent(
    documentId,
    Number(version),
  );
  if (!result.found) throw new NotFoundError("Không tìm thấy version");
  return result;
};

export const deleteDocument = async (req: FastifyRequest) => {
  const { documentId } = req.params as { documentId: string };
  return docService.removeDocument(documentId);
};
