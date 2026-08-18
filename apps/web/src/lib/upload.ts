import api from "@/lib/http";

const MAX_IMAGE_BYTES = 2 * 1024 * 1024;

const MIME_BY_EXT: Record<string, string> = {
  png: "image/png",
  jpg: "image/jpeg",
  jpeg: "image/jpeg",
  gif: "image/gif",
  webp: "image/webp",
  svg: "image/svg+xml",
};

/**
 * Upload ảnh (avatar/agent logo) lên MinIO theo presigned POST flow:
 *   1. GET /api/upload/presigned → { url, fields, key }
 *   2. POST thẳng lên MinIO
 *   3. GET /api/upload/download/:key → { url } (presigned download)
 * Trả về URL dùng để hiển thị / lưu vào avatar_url.
 */
export async function uploadImage(file: File): Promise<string> {
  if (!file.type.startsWith("image/")) {
    throw new Error("Chỉ chấp nhận file ảnh");
  }
  if (file.size > MAX_IMAGE_BYTES) {
    throw new Error("Ảnh tối đa 2MB");
  }

  const ext = (file.name.split(".").pop() || "png").toLowerCase();
  const contentType = MIME_BY_EXT[ext] ?? file.type ?? "image/png";

  const presigned = await api.get<{
    url: string;
    fields: Record<string, string>;
    key: string;
  }>(
    `/api/upload/presigned?category=images&ext=${ext}&contentType=${encodeURIComponent(contentType)}`,
  );

  const form = new FormData();
  Object.entries(presigned.fields).forEach(([k, v]) => form.append(k, v));
  form.append("file", file);

  const uploadRes = await fetch(presigned.url, { method: "POST", body: form });
  if (!uploadRes.ok) {
    throw new Error("Tải ảnh lên thất bại");
  }

  const download = await api.get<{ key: string; url: string }>(
    `/api/upload/download/${presigned.key}`,
  );
  return download.url;
}
