import {
  createConversation as createConversationRepo,
  listConversations as listConversationsRepo,
  getMessages as getMessagesRepo,
  addMessage,
  deleteConversation as deleteConversationRepo,
} from "../repositories";
import { createAgentClient, type AgentClient } from "../../../agent/client";
import type { MessageRole } from "../../../schemas/message";

type ChatMessage = { role: MessageRole; content: string };

// Agent backend (LangGraph in-process hôm nay; P12 swap sang agent-go qua config).
const defaultAgent = createAgentClient();

/**
 * Pure helper: lọc bỏ message role "tool", map về {role, content} cho agent.
 * (tách riêng để test không cần I/O)
 */
export const toAnthropicMessages = (history: ChatMessage[]) =>
  history
    .filter((m) => m.role === "user" || m.role === "assistant")
    .map((m) => ({ role: m.role as "user" | "assistant", content: m.content }));

// ----- CRUD hội thoại -----
export const createConversation = (firstMessage: string) =>
  createConversationRepo(firstMessage);

export const listConversations = () => listConversationsRepo();

export const getConversationMessages = (id: string) => getMessagesRepo(id);

export const deleteConversation = (id: string) => deleteConversationRepo(id);

// Lưu tin nhắn user (tách riêng để controller gọi TRƯỚC khi mở SSE)
export const appendUserMessage = (conversationId: string, content: string) =>
  addMessage(conversationId, "user", content);

/**
 * Chạy agent (LangGraph) cho hội thoại, yield từng AgentEvent để controller
 * đẩy ra SSE. Tự lưu câu trả lời assistant vào DB khi stream xong.
 */
export async function* streamReply(
  conversationId: string,
  signal?: AbortSignal,
  agent: AgentClient = defaultAgent,
) {
  const raw = (await getMessagesRepo(
    conversationId,
  )) as unknown as ChatMessage[];
  const history = toAnthropicMessages(raw);

  let full = "";
  try {
    for await (const ev of agent.stream(history, { signal })) {
      if (ev.type === "text") full += ev.text;
      yield ev;
    }
  } finally {
    // Lưu cả khi bị abort giữa chừng: giữ lại phần đã sinh, tránh mất câu trả lời
    // dài khi user lỡ đổi trang. Bỏ qua nếu rỗng để không làm nhiễu lượt sau.
    if (full.trim().length > 0) {
      await addMessage(conversationId, "assistant", full);
    }
  }
}
