export type Conversation = { _id: string; title: string };
export type Message = { _id?: string; role: string; content: string };

export const createConversation = async (
  firstMessage: string,
): Promise<Conversation> => {
  const response = await fetch("/api/conversations", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ firstMessage }),
  });
  return response.json();
};

export const listConversations = async (): Promise<Conversation[]> => {
  const response = await fetch("/api/conversations");
  return response.json();
};

export const getMessages = async (id: string): Promise<Message[]> => {
  const response = await fetch(`/api/conversations/${id}/messages`);
  return response.json();
};

export const deleteConversation = async (id: string): Promise<void> => {
  await fetch(`/api/conversations/${id}`, { method: "DELETE" });
};

// Một event SSE từ agent: token (text), hoặc tool_start/tool_end, hoặc done
export type ChatEvent = {
  token?: string;
  type?: "text" | "tool_start" | "tool_end";
  name?: string;
  done?: boolean;
};

// Gửi tin nhắn và stream event (token + tool) về qua callback
export const streamChat = async (
  conversationId: string,
  content: string,
  onEvent: (e: ChatEvent) => void,
): Promise<void> => {
  const response = await fetch(`/api/conversations/${conversationId}/chat`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ content }),
  });
  const reader = response.body!.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    const lines = buffer.split("\n\n");
    buffer = lines.pop() ?? "";
    for (const line of lines) {
      if (!line.startsWith("data: ")) continue;
      onEvent(JSON.parse(line.slice(6)) as ChatEvent);
    }
  }
};

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
