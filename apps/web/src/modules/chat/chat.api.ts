import { http } from "@/shared/api/http";

export type Conversation = { _id: string; title: string };
export type Message = {
  _id?: string;
  role: "user" | "assistant";
  content: string;
};

export const createConversation = (firstMessage: string) =>
  http.post<Conversation>("/conversations", { firstMessage });

export const listConversations = () =>
  http.get<Conversation[]>("/conversations");

export const getMessages = (id: string) =>
  http.get<Message[]>(`/conversations/${id}/messages`);

export const deleteConversation = (id: string) =>
  http.delete<void>(`/conversations/${id}`);

// Một event SSE từ agent: token (text), tool_start/tool_end, error, hoặc done
export type ChatEvent = {
  token?: string;
  type?: "text" | "tool_start" | "tool_end" | "error";
  name?: string;
  message?: string;
  done?: boolean;
};

// Gửi tin nhắn và stream event (token + tool) về qua callback.
// - signal: hủy stream khi đổi hội thoại / unmount / bấm "Dừng".
export const streamChat = async (
  conversationId: string,
  content: string,
  onEvent: (e: ChatEvent) => void,
  signal?: AbortSignal,
): Promise<void> => {
  const response = await http.stream(
    `/conversations/${conversationId}/chat`,
    { content },
    { signal },
  );
  if (!response.body) throw new Error("Không nhận được stream từ server");

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split("\n\n");
      buffer = lines.pop() ?? "";
      for (const line of lines) {
        if (!line.startsWith("data: ")) continue;
        // Bỏ qua dòng hỏng (vd keep-alive comment) thay vì để cả stream chết.
        try {
          onEvent(JSON.parse(line.slice(6)) as ChatEvent);
        } catch {
          // ignore malformed SSE line
        }
      }
    }
    // Flush ký tự multibyte còn sót ở cuối stream.
    buffer += decoder.decode();
  } finally {
    reader.releaseLock();
  }
};
