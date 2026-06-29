import { http } from "@/shared/api/http";

export type Conversation = { _id: string; title: string };
export type Message = { _id?: string; role: string; content: string };

export const createConversation = (firstMessage: string) =>
  http.post<Conversation>("/conversations", { firstMessage });

export const listConversations = () =>
  http.get<Conversation[]>("/conversations");

export const getMessages = (id: string) =>
  http.get<Message[]>(`/conversations/${id}/messages`);

export const deleteConversation = (id: string) =>
  http.delete<void>(`/conversations/${id}`);

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
  const response = await http.stream(`/conversations/${conversationId}/chat`, {
    content,
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
