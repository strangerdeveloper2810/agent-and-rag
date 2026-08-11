import {
  createConversation as createConversationRepo,
  listConversations as listConversationsRepo,
  getMessages as getMessagesRepo,
  addMessage,
  deleteConversation as deleteConversationRepo,
} from "../repositories";
import {
  createAgentClient,
  type AgentClient,
} from "../../../agent/client/index";
import type { AgentEvent } from "../../../agent/graph/index";
import { config } from "../../../config";
import type { MessageRole } from "../../../schemas/message";
import { getChatCache, setChatCache } from "../../../common/cache/index";

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
export const appendUserMessage = (
  conversationId: string,
  content: string,
  attachments?: Array<{
    type: string;
    name: string;
    size?: number;
    mimeType: string;
    data?: string;
    thumbnail?: string;
  }>,
) => {
  const metaAttachments = attachments?.map((a) => ({
    type: (a.type === "image" ? "image" : "file") as "image" | "file",
    name: a.name,
    size: a.size ?? 0,
    mimeType: a.mimeType,
    thumbnail:
      a.thumbnail ||
      (a.data
        ? a.data.startsWith("data:")
          ? a.data
          : `data:${a.mimeType};base64,${a.data}`
        : undefined),
  }));

  return addMessage(
    conversationId,
    "user",
    content,
    undefined,
    metaAttachments,
  );
};

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

  const model =
    config.LLM_PROVIDER === "google"
      ? config.GOOGLE_MODEL
      : config.CLAUDE_MODEL;
  const cacheInput = {
    model,
    temperature: 1.0,
    messages: history as unknown as Record<string, unknown>[],
  };

  let metadataResolve!: (meta: StreamMetadata) => void;
  const metadata = new Promise<StreamMetadata>((resolve) => {
    metadataResolve = resolve;
  });

  async function* generator(): AsyncGenerator<AgentEvent> {
    // 1. Kiểm tra chat cache trước khi gọi agent
    try {
      const cached = await getChatCache(cacheInput);
      if (cached) {
        console.log(`[chat-cache] hit (model=${model})`);
        full = cached;
        yield { type: "text", text: cached };
        metadataResolve({ backend: backend + "+cache", tokensUsed: 0 });
        await addMessage(conversationId, "assistant", cached);
        return;
      }
    } catch {
      // Redis không khả dụng → fallback không cache
    }

    try {
      for await (const ev of agent.stream(history, { signal, attachments })) {
        if (ev.type === "text") full += ev.text;

        if (ev.type === "done") {
          if (ev.agent) backend = ev.agent;
          if (ev.tokens) tokensUsed = ev.tokens;
        }

        yield ev;
      }
    } finally {
      metadataResolve({ backend, tokensUsed });

      if (full.trim().length > 0) {
        await addMessage(conversationId, "assistant", full);

        // Lưu response vào chat cache
        try {
          await setChatCache(cacheInput, full);
          console.log(`[chat-cache] saved (model=${model})`);
        } catch {
          // Redis không khả dụng → bỏ qua
        }
      }
    }
  }

  return { events: generator(), metadata };
}
