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
 * Upload ảnh (avatar/agent logo) lên server / MinIO qua API Gateway:
 *   POST /api/upload?category=images (multipart/form-data)
 *
 * Trả về relative URL `/api/upload/file/...` dùng để hiển thị / lưu vào avatar_url.
 * Đảm bảo hoạt động qua HTTPS trên production (tránh Mixed Content và lỗi DNS docker minio:9000).
 */
export async function uploadImage(file: File): Promise<string> {
  if (!file.type.startsWith("image/")) {
    throw new Error("Chỉ chấp nhận file ảnh");
  }
  if (file.size > MAX_IMAGE_BYTES) {
    throw new Error("Ảnh tối đa 2MB");
  }

  const formData = new FormData();
  formData.append("file", file);

  const res = await fetch("/api/upload?category=images", {
    method: "POST",
    body: formData,
    credentials: "include",
  });

  if (!res.ok) {
    const errBody = await res.json().catch(() => ({}));
    throw new Error(errBody.message || errBody.error || "Tải ảnh lên thất bại");
  }

  const data = (await res.json()) as { url: string; key: string };
  return data.url;
}

