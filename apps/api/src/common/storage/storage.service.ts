/**
 * Storage Service — upload, download, delete file với MinIO/S3.
 *
 * Mỗi tenant có prefix riêng: {tenantId}/{category}/{filename}
 * - ảnh:        {tenantId}/images/{uuid}.{ext}
 * - documents:  {tenantId}/docs/{uuid}.{ext}
 * - notes:      {tenantId}/notes/{uuid}.md
 * - memories:   {tenantId}/memories/{uuid}.json
 */
import {
  PutObjectCommand,
  GetObjectCommand,
  DeleteObjectCommand,
  HeadObjectCommand,
} from "@aws-sdk/client-s3";
import { getSignedUrl } from "@aws-sdk/s3-request-presigner";
import { createPresignedPost } from "@aws-sdk/s3-presigned-post";
import { getS3Client } from "./s3.client";
import { config } from "../../config";

export type StorageCategory = "images" | "docs" | "notes" | "memories";

export interface UploadResult {
  key: string;
  url: string;
}

export interface PresignedPostResult {
  url: string;
  fields: Record<string, string>; // form fields client cần gửi kèm
  key: string;
}

export interface StoredFile {
  key: string;
  size: number;
  contentType: string;
  lastModified: Date;
}

const BUCKET = config.S3_BUCKET;
const URL_EXPIRY = 3600; // Presigned download URL hết hạn sau 1 giờ
const UPLOAD_EXPIRY = 300; // Presigned upload URL hết hạn sau 5 phút

const s3 = () => getS3Client();

// ── Helpers ──

/** Tạo object key theo chuẩn {tenantId}/{category}/{filename} */
const objectKey = (
  tenantId: string,
  category: StorageCategory,
  filename: string,
): string => `${tenantId}/${category}/${filename}`;

/** Đoán content-type từ extension. */
export const guessContentType = (filename: string): string => {
  const ext = filename.split(".").pop()?.toLowerCase();
  const map: Record<string, string> = {
    png: "image/png",
    jpg: "image/jpeg",
    jpeg: "image/jpeg",
    gif: "image/gif",
    webp: "image/webp",
    svg: "image/svg+xml",
    pdf: "application/pdf",
    doc: "application/msword",
    docx: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
    md: "text/markdown",
    json: "application/json",
    txt: "text/plain",
    csv: "text/csv",
  };
  return map[ext ?? ""] ?? "application/octet-stream";
};

// ── Public API ──

/**
 * Upload file lên MinIO.
 * @returns key + presigned URL để download (có thời hạn).
 */
export const uploadFile = async (
  tenantId: string,
  category: StorageCategory,
  filename: string,
  body: Buffer | Uint8Array | string,
  contentType?: string,
): Promise<UploadResult> => {
  const key = objectKey(tenantId, category, filename);
  const ct = contentType ?? guessContentType(filename);

  await s3().send(
    new PutObjectCommand({
      Bucket: BUCKET,
      Key: key,
      Body: body,
      ContentType: ct,
    }),
  );

  const url = await getSignedUrl(
    s3(),
    new GetObjectCommand({ Bucket: BUCKET, Key: key }),
    { expiresIn: URL_EXPIRY },
  );

  return { key, url };
};

/**
 * Lấy presigned URL để download file (dùng cho ảnh, doc).
 * Không tải file về server — trả URL để client download trực tiếp từ MinIO.
 */
export const getDownloadUrl = async (key: string): Promise<string> => {
  return getSignedUrl(
    s3(),
    new GetObjectCommand({ Bucket: BUCKET, Key: key }),
    { expiresIn: URL_EXPIRY },
  );
};

/**
 * Lấy stream của file từ MinIO để proxy stream trực tiếp về client qua API gateway.
 */
export const getFileStream = async (
  key: string,
): Promise<{
  stream: unknown;
  contentType: string;
  contentLength?: number;
} | null> => {
  try {
    const res = await s3().send(
      new GetObjectCommand({ Bucket: BUCKET, Key: key }),
    );
    if (!res.Body) {
      return null;
    }
    return {
      stream: res.Body,
      contentType: res.ContentType || guessContentType(key),
      contentLength: res.ContentLength,
    };
  } catch (err) {
    if (
      (err as { $metadata?: { httpStatusCode?: number } }).$metadata
        ?.httpStatusCode === 404
    ) {
      return null;
    }
    throw err;
  }
};

/**
 * Download file content từ MinIO về server.
 */
export const downloadFile = async (key: string): Promise<Buffer | null> => {
  try {
    const res = await s3().send(
      new GetObjectCommand({ Bucket: BUCKET, Key: key }),
    );
    const chunks: Uint8Array[] = [];
    if (res.Body) {
      for await (const chunk of res.Body as AsyncIterable<Uint8Array>) {
        chunks.push(chunk);
      }
    }
    return Buffer.concat(chunks);
  } catch (err) {
    if (
      (err as { $metadata?: { httpStatusCode?: number } }).$metadata
        ?.httpStatusCode === 404
    ) {
      return null;
    }
    throw err;
  }
};

/** Xoá file khỏi MinIO. */
export const deleteFile = async (key: string): Promise<void> => {
  await s3().send(new DeleteObjectCommand({ Bucket: BUCKET, Key: key }));
};

/** Kiểm tra file có tồn tại không. */
export const fileExists = async (key: string): Promise<boolean> => {
  try {
    await s3().send(new HeadObjectCommand({ Bucket: BUCKET, Key: key }));
    return true;
  } catch {
    return false;
  }
};

/**
 * Tạo presigned POST URL cho client upload TRỰC TIẾP lên MinIO.
 * Client upload thẳng → không qua API server → không tốn RAM server.
 *
 * Flow:
 * 1. Client gọi GET /api/upload/presigned?category=images&ext=png
 * 2. Server trả về URL + form fields
 * 3. Client POST form thẳng đến MinIO URL đó
 * 4. Client gọi lại API để confirm upload hoàn tất
 */
export const createUploadUrl = async (
  tenantId: string,
  category: StorageCategory,
  filename: string,
  contentType?: string,
): Promise<PresignedPostResult> => {
  const key = objectKey(tenantId, category, filename);
  const ct = contentType ?? guessContentType(filename);

  const result = await createPresignedPost(s3(), {
    Bucket: BUCKET,
    Key: key,
    Conditions: [
      ["content-length-range", 0, 25 * 1024 * 1024], // max 25MB
      ["eq", "$Content-Type", ct],
    ],
    Fields: {
      "Content-Type": ct,
    },
    Expires: UPLOAD_EXPIRY,
  });

  return {
    url: result.url,
    fields: result.fields,
    key,
  };
};

/**
 * Lấy public URL trực tiếp từ MinIO (không hết hạn).
 * Yêu cầu: bucket policy cho phép public read trên path images/.
 * Nếu chưa set policy, fallback về presigned URL.
 */
export const getPublicUrl = async (key: string): Promise<string> => {
  // Thử presigned URL — an toàn nhất cho mọi trường hợp
  return getSignedUrl(
    s3(),
    new GetObjectCommand({ Bucket: BUCKET, Key: key }),
    { expiresIn: URL_EXPIRY },
  );
};

