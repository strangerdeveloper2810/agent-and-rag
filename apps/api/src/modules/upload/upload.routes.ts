/**
 * Upload routes — 2 kiểu upload:
 *
 * 1. Presigned POST (khuyến nghị cho ảnh, doc lớn):
 *    Client GET /api/upload/presigned → nhận URL + form fields
 *    → POST thẳng lên MinIO (không qua API server, không tốn RAM)
 *
 * 2. Server-side upload (cho file nhỏ: notes, memories agent tạo):
 *    Client POST /api/upload → API buffer file → MinIO
 *
 * Auth guard sẽ được thêm ở Phase 5 (BFF refactor).
 */
import type { FastifyInstance, FastifyRequest } from "fastify";
import * as uploadService from "./upload.service";
import { BadRequestError } from "../../lib/errors";

const getTenantId = (req: FastifyRequest): string =>
  (req as unknown as Record<string, unknown>).tenantId as string ?? "default";

export const uploadRoutes = async (app: FastifyInstance): Promise<void> => {
  /**
   * GET /api/upload/presigned?category=images&ext=png&contentType=image/png
   *
   * Tạo presigned POST URL cho client upload thẳng lên MinIO.
   * Trả về { url, fields, key } — client dùng HTML form hoặc fetch để POST.
   *
   * Cách dùng từ browser:
   *   const { url, fields } = await fetch('/api/upload/presigned?...').then(r=>r.json());
   *   const form = new FormData();
   *   Object.entries(fields).forEach(([k,v]) => form.append(k,v));
   *   form.append('file', myFile);
   *   await fetch(url, { method: 'POST', body: form });
   */
  app.get("/api/upload/presigned", async (req, reply) => {
    const q = req.query as Record<string, string | undefined>;
    const tenantId = getTenantId(req);
    const category = uploadService.parseCategory(q.category);
    const ext = q.ext?.replace(/^\./, "") ?? "bin";

    const result = await uploadService.createPresignedUpload({
      tenantId,
      category,
      ext,
      contentType: q.contentType,
    });

    return reply.status(200).send(result);
  });

  /**
   * POST /api/upload?category=images
   *
   * Server-side upload: file → API buffer → MinIO.
   * Dùng cho file nhỏ (notes, memories) hoặc khi client không hỗ trợ presigned POST.
   */
  app.post("/api/upload", async (req, reply) => {
    const file = await req.file();
    if (!file) {
      throw new BadRequestError("Thiếu file.");
    }

    const tenantId = getTenantId(req);
    const category = uploadService.parseCategory(
      (req.query as Record<string, unknown>)?.category,
    );
    const buf = await file.toBuffer();

    const record = await uploadService.uploadFileServer({
      tenantId,
      originalName: file.filename,
      mimeType: file.mimetype,
      size: buf.length,
      buffer: buf,
      category,
    });

    return reply.status(201).send({
      _id: record._id,
      key: record.key,
      url: record.url,
      filename: record.filename,
      originalName: record.originalName,
      size: record.size,
      category: record.category,
    });
  });

  /**
   * GET /api/upload
   * Liệt kê danh sách upload của tenant hiện tại (auth required).
   * Query: ?category=images (tuỳ chọn)
   */
  app.get("/api/upload", async (req, reply) => {
    const tenantId = getTenantId(req);
    const q = req.query as Record<string, string | undefined>;
    const records = await uploadService.listUploads(tenantId, q.category);
    return reply.status(200).send(records);
  });

  /**
   * GET /api/upload/download/:tenantId/:category/:filename
   * Lấy URL để download/view file (presigned, 1h).
   */
  app.get(
    "/api/upload/download/:tenantId/:category/:filename",
    async (req, reply) => {
      const { tenantId, category, filename } = req.params as {
        tenantId: string;
        category: string;
        filename: string;
      };
      const key = `${tenantId}/${category}/${filename}`;
      const url = await uploadService.getDownloadUrl(key);
      return reply.status(200).send({ key, url });
    },
  );

  /**
   * DELETE /api/upload/:key
   * Xoá file khỏi MinIO và database record.
   */
  app.delete("/api/upload/:key", async (req, reply) => {
    const tenantId = getTenantId(req);
    const { key } = req.params as { key: string };
    await uploadService.removeUpload(tenantId, key);
    return reply.status(200).send({ deleted: key });
  });
};
