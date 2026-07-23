import type { FastifyRequest } from "fastify";
import * as docService from "../services";
import { BadRequestError, NotFoundError } from "../../../lib/errors";

// Upload file MỚI (.txt/.md/.pdf) → version 1
export const uploadDocument = async (req: FastifyRequest) => {
  const file = await req.file();
  if (!file) throw new BadRequestError("Thiếu file");
  const buffer = await file.toBuffer();
  return docService.ingestUpload(file.filename, buffer);
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
