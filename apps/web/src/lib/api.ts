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
export type DocumentInfo = { source: string; chunks: number };

export const listDocuments = async (): Promise<DocumentInfo[]> => {
  const response = await fetch("/api/documents");
  return response.json();
};

export const uploadDocument = async (
  file: File,
): Promise<{ source: string; chunks: number }> => {
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

export const deleteDocument = async (source: string): Promise<void> => {
  await fetch(`/api/documents/${encodeURIComponent(source)}`, {
    method: "DELETE",
  });
};
