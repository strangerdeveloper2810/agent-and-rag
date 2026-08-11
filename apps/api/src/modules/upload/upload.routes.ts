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
import type { FastifyInstance } from "fastify";
import {
  uploadFile,
  getPublicUrl,
  deleteFile,
  createUploadUrl,
} from "../../common/storage/storage.service";
import type { StorageCategory } from "../../common/storage/storage.service";
import { v4 as uuid } from "uuid";

// TODO Phase 5: thay placeholder bằng req.tenantId từ authGuard
const PLACEHOLDER_TENANT = "default";

const VALID_CATEGORIES: StorageCategory[] = [
  "images",
  "docs",
  "notes",
  "memories",
];

const parseCategory = (cat?: unknown): StorageCategory => {
  const c = typeof cat === "string" ? cat : "images";
  return VALID_CATEGORIES.includes(c as StorageCategory)
    ? (c as StorageCategory)
    : "images";
};

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
    const category = parseCategory(q.category);
    const ext = q.ext?.replace(/^\./, "") ?? "bin";
    const filename = `${uuid()}.${ext}`;
    const contentType = q.contentType;

    const result = await createUploadUrl(
      PLACEHOLDER_TENANT,
      category,
      filename,
      contentType,
    );

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
      return reply.status(400).send({ error: "Thiếu file." });
    }

    const category = parseCategory(
      (req.query as Record<string, unknown>)?.category,
    );
    const ext = file.filename.split(".").pop() ?? "bin";
    const filename = `${uuid()}.${ext}`;
    const buf = await file.toBuffer();

    const result = await uploadFile(
      PLACEHOLDER_TENANT,
      category,
      filename,
      buf,
      file.mimetype,
    );

    return reply.status(201).send({
      key: result.key,
      url: result.url,
      filename: file.filename,
      size: buf.length,
      category,
    });
  });

  /**
   * GET /api/upload/:tenantId/:category/:filename
   * Lấy URL để download/view file (presigned, 1h).
   */
  app.get(
    "/api/upload/:tenantId/:category/:filename",
    async (req, reply) => {
      const { tenantId, category, filename } = req.params as {
        tenantId: string;
        category: string;
        filename: string;
      };
      const key = `${tenantId}/${category}/${filename}`;
      const url = await getPublicUrl(key);
      return reply.status(200).send({ key, url });
    },
  );

  /**
   * DELETE /api/upload/:tenantId/:category/:filename
   * Xoá file khỏi MinIO.
   */
  app.delete(
    "/api/upload/:tenantId/:category/:filename",
    async (req, reply) => {
      const { tenantId, category, filename } = req.params as {
        tenantId: string;
        category: string;
        filename: string;
      };
      const key = `${tenantId}/${category}/${filename}`;
      await deleteFile(key);
      return reply.status(200).send({ deleted: key });
    },
  );
};
