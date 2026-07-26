/**
 * S3 client — singleton wrapper cho MinIO (S3-compatible).
 *
 * Dùng @aws-sdk/client-s3 vì MinIO API tương thích hoàn toàn với S3.
 * Production chỉ cần đổi S3_ENDPOINT + credentials là sang AWS S3 được.
 */
import { S3Client } from "@aws-sdk/client-s3";
import { config } from "../../config";

let s3: S3Client | null = null;

/** Lấy S3 client instance (singleton, idempotent). */
export const getS3Client = (): S3Client => {
  if (!s3) {
    s3 = new S3Client({
      endpoint: config.S3_ENDPOINT,
      region: config.S3_REGION,
      credentials: {
        accessKeyId: config.S3_ACCESS_KEY,
        secretAccessKey: config.S3_SECRET_KEY,
      },
      forcePathStyle: true, // MinIO yêu cầu path-style (không virtual host)
      requestChecksumCalculation: "WHEN_REQUIRED",
      responseChecksumValidation: "WHEN_REQUIRED",
    });
  }
  return s3;
};
