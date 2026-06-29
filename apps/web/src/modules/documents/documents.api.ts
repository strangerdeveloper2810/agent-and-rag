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

export const listDocuments = async (): Promise<DocumentInfo[]> => {
  const response = await fetch("/api/documents");
  return response.json();
};

export const uploadDocument = async (file: File): Promise<DocumentInfo> => {
  const form = new FormData();
  form.append("file", file);
  const response = await fetch("/api/documents/upload", {
    method: "POST",
    body: form,
  });
  if (!response.ok) {
    const data = await response.json().catch(() => ({}));
    throw new Error(data.error ?? "Upload thất bại");
  }
  return response.json();
};

// Cập nhật tài liệu đã có → tạo version mới (file mới có thể khác tên)
export const updateDocument = async (
  documentId: string,
  file: File,
): Promise<DocumentInfo> => {
  const form = new FormData();
  form.append("file", file);
  const response = await fetch(`/api/documents/${documentId}`, {
    method: "PUT",
    body: form,
  });
  if (!response.ok) {
    const data = await response.json().catch(() => ({}));
    throw new Error(data.error ?? "Cập nhật thất bại");
  }
  return response.json();
};

export const getVersions = async (
  documentId: string,
): Promise<DocumentVersion[]> => {
  const response = await fetch(`/api/documents/${documentId}/versions`);
  return response.json();
};

export const getVersionContent = async (
  documentId: string,
  version: number,
): Promise<VersionContent> => {
  const response = await fetch(
    `/api/documents/${documentId}/versions/${version}`,
  );
  return response.json();
};

export const deleteDocument = async (documentId: string): Promise<void> => {
  await fetch(`/api/documents/${documentId}`, {
    method: "DELETE",
  });
};
