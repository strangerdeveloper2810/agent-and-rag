import { http } from "@/shared/api/http";

// ----- Documents (RAG) -----
export type DocumentInfo = {
  documentId: string;
  source: string;
  version: number;
  chunks: number;
};

export type DocumentVersion = {
  version: number;
  source: string;
  isLatest: boolean;
};

export type VersionContent = {
  found: boolean;
  documentId: string;
  version: number;
  source: string;
  content: string;
  isLatest: boolean;
};

const fileForm = (file: File) => {
  const form = new FormData();
  form.append("file", file);
  return form;
};

// Kết quả nạp cho MỘT file trong lô upload (best-effort).
export type UploadResult =
  | ({ filename: string; ok: true } & DocumentInfo)
  | { filename: string; ok: false; error: string };

export const listDocuments = () => http.get<DocumentInfo[]>("/documents");

// Upload 1–7 file cùng lúc → trả kết quả từng file.
export const uploadDocuments = (files: File[]) => {
  const form = new FormData();
  for (const f of files) form.append("file", f);
  return http.post<{ results: UploadResult[] }>("/documents/upload", form);
};

// Cập nhật tài liệu đã có → tạo version mới (file mới có thể khác tên)
export const updateDocument = (documentId: string, file: File) =>
  http.put<DocumentInfo>(`/documents/${documentId}`, fileForm(file));

export const getVersions = (documentId: string) =>
  http.get<DocumentVersion[]>(`/documents/${documentId}/versions`);

export const getVersionContent = (documentId: string, version: number) =>
  http.get<VersionContent>(`/documents/${documentId}/versions/${version}`);

export const deleteDocument = (documentId: string) =>
  http.delete<void>(`/documents/${documentId}`);
