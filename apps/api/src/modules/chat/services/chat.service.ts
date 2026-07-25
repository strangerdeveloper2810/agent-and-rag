import {
  createConversation as createConversationRepo,
  listConversations as listConversationsRepo,
  getMessages as getMessagesRepo,
  addMessage,
  deleteConversation as deleteConversationRepo,
} from "../repositories";
import { createAgentClient, type AgentClient } from "../../../agent/client/index";
import type { AgentEvent } from "../../../agent/graph/index";
import { config } from "../../../config";
import type { MessageRole } from "../../../schemas/message";

type ChatMessage = { role: MessageRole; content: string };

/** Kết quả metadata sau khi agent stream hoàn tất. */
export interface StreamMetadata {
  /** Tên agent backend đã xử lý (vd "langgraph", "go-default"). */
  backend: string;
  /** Tổng token đã dùng (input + output), nếu agent trả về. */
  tokensUsed: number;
}

/** Kết quả stream: generator + metadata promise hứa khi stream xong. */
export interface StreamResult {
  events: AsyncGenerator<AgentEvent>;
  /** Resolve khi stream kết thúc (thành công hoặc lỗi). */
  metadata: Promise<StreamMetadata>;
}

// Agent backend singleton.
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

/** Lưu tin nhắn user (gọi TRƯỚC khi mở SSE để validate sớm). */
export const appendUserMessage = (conversationId: string, content: string) =>
  addMessage(conversationId, "user", content);

/**
 * Chạy agent cho hội thoại, yield từng AgentEvent để controller đẩy ra SSE.
 * Tự lưu câu trả lời assistant vào DB khi stream xong.
 *
 * Trả về StreamResult để controller có thể đợi metadata (agent name, tokens)
 * và gửi event `done` cuối cùng với thông tin đó.
 */
export async function streamReply(
  conversationId: string,
  signal?: AbortSignal,
  agent: AgentClient = defaultAgent,
  attachments?: Array<{
    type: string;
    name: string;
    data: string;
    mimeType: string;
  }>,
): Promise<StreamResult> {
  const raw = (await getMessagesRepo(
    conversationId,
  )) as unknown as ChatMessage[];
  const history = toAnthropicMessages(raw);

  let full = "";
  let tokensUsed = 0;
  let backend: string = config.AGENT_BACKEND;

  let metadataResolve!: (meta: StreamMetadata) => void;
  const metadata = new Promise<StreamMetadata>((resolve) => {
    metadataResolve = resolve;
  });

  async function* generator(): AsyncGenerator<AgentEvent> {
    try {
      for await (const ev of agent.stream(history, { signal, attachments })) {
        if (ev.type === "text") full += ev.text;

        // Ghi nhận metadata từ event done của Go agent.
        if (ev.type === "done") {
          if (ev.agent) backend = ev.agent;
          if (ev.tokens) tokensUsed = ev.tokens;
        }

        yield ev;
      }
    } finally {
      // Luôn trả metadata (kể cả khi bị abort giữa chừng).
      metadataResolve({ backend, tokensUsed });

      // Lưu cả khi bị abort: giữ lại phần đã sinh, tránh mất câu trả lời
      // dài khi user lỡ đổi trang.
      if (full.trim().length > 0) {
        await addMessage(conversationId, "assistant", full);
      }
    }
  }

  return { events: generator(), metadata };
}
