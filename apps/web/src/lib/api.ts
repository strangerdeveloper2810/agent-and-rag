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

// Gửi tin nhắn và stream token về qua callback
export const streamChat = async (
  conversationId: string,
  content: string,
  onToken: (token: string) => void,
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
      const data = JSON.parse(line.slice(6));
      if (data.token) onToken(data.token);
    }
  }
};
