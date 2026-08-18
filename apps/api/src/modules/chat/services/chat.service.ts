import {
  createConversation as createConversationRepo,
  listConversations as listConversationsRepo,
  getMessages as getMessagesRepo,
  addMessage,
  appendToLastAssistantMessage,
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
import { BadRequestError } from "../../../lib/errors";

type ChatMessage = { role: MessageRole; content: string };

/** Kết quả metadata sau khi agent stream hoàn tất. */
export interface StreamMetadata {
  /** Tên agent backend đã xử lý (vd "langgraph", "go-default"). */
  backend: string;
  /** Tổng token đã dùng (input + output), nếu agent trả về. */
  tokensUsed: number;
  /** true khi câu trả lời bị cắt vì chạm giới hạn output token. */
  truncated: boolean;
  /** Kích thước ước tính (token) của context ở CUỐI lượt (Go agent). */
  contextTokens?: number;
  /** Ngân sách token context (Go agent). 0/undefined = không giới hạn. */
  contextBudget?: number;
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
  tenantId?: string,
): Promise<StreamResult> {
  const raw = (await getMessagesRepo(
    conversationId,
  )) as unknown as ChatMessage[];
  const history = toAnthropicMessages(raw);

  let full = "";
  let tokensUsed = 0;
  let truncated = false;
  let backend: string = config.AGENT_BACKEND;
  let contextTokens: number | undefined;
  let contextBudget: number | undefined;

  // Định danh model dùng cho CACHE KEY phải phản ánh thứ THỰC SỰ sinh câu trả
  // lời. Trước đây luôn lấy GOOGLE_MODEL/CLAUDE_MODEL của BFF, trong khi với
  // AGENT_BACKEND=go thì Go agent sinh câu trả lời bằng provider/model khác
  // hoàn toàn (log dev: cache ghi "gemini-3.1-flash-lite" nhưng thực tế DeepSeek
  // trả lời). Hệ quả: cache key không đổi khi đổi model/backend → trả về câu
  // trả lời cũ của model cũ.
  //
  // Hạn chế còn lại: BFF không biết tên model bên trong Go agent, nên khi đổi
  // model Ở PHÍA GO hãy bump CHAT_CACHE_VERSION để vô hiệu cache.
  const model =
    config.AGENT_BACKEND === "go"
      ? `go-agent@${config.AGENT_GO_URL}`
      : `${config.LLM_PROVIDER}:${
          config.LLM_PROVIDER === "google"
            ? config.GOOGLE_MODEL
            : config.CLAUDE_MODEL
        }`;
  const cacheInput = {
    // CHAT_CACHE_VERSION là van xả tay: bump giá trị này để vô hiệu toàn bộ
    // cache sau khi đổi model bên Go agent hoặc đổi system prompt.
    model: `v${config.CHAT_CACHE_VERSION}|${model}`,
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
        metadataResolve({
          backend: backend + "+cache",
          tokensUsed: 0,
          truncated: false,
        });
        await addMessage(conversationId, "assistant", cached);
        return;
      }
    } catch {
      // Redis không khả dụng → fallback không cache
    }

    try {
      for await (const ev of agent.stream(history, {
        signal,
        attachments,
        tenantId,
      })) {
        if (ev.type === "text") full += ev.text;

        if (ev.type === "truncated") truncated = true;

        if (ev.type === "done") {
          if (ev.agent) backend = ev.agent;
          if (ev.tokens) tokensUsed = ev.tokens;
          if (ev.truncated) truncated = true;
          if (ev.contextTokens !== undefined) contextTokens = ev.contextTokens;
          if (ev.contextBudget !== undefined) contextBudget = ev.contextBudget;
        }

        yield ev;
      }
    } finally {
      metadataResolve({ backend, tokensUsed, truncated, contextTokens, contextBudget });

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

/**
 * Instruction NỘI BỘ gửi cho agent khi tiếp tục 1 câu trả lời bị cắt — KHÔNG
 * BAO GIỜ hiển thị cho user hay lưu vào DB (khác prompt "Tiếp tục..." cũ ở
 * FE, vốn là 1 user message thật, tạo ra cặp user+assistant message MỚI).
 * Vì response được NỐI TRỰC TIẾP vào cuối nội dung cũ
 * (appendToLastAssistantMessage), model phải TUYỆT ĐỐI không thêm lời dẫn/
 * tiêu đề/mở lại code fence — nếu không nội dung merge sẽ hỏng (đúng lỗi đã
 * thấy trong log dev: model tự thêm "Dưới đây là phần tiếp theo:...").
 */
const CONTINUE_INSTRUCTION =
  "Phản hồi trước của bạn bị cắt giữa chừng do chạm giới hạn độ dài. Hãy viết " +
  "TIẾP TỤC từ CHÍNH XÁC vị trí bị cắt, sao cho khi nối trực tiếp vào cuối " +
  "nội dung cũ (không thêm khoảng trắng hay xuống dòng thừa ở đầu) tạo thành " +
  "1 văn bản liền mạch duy nhất. TUYỆT ĐỐI KHÔNG lặp lại nội dung đã có, " +
  "KHÔNG thêm lời dẫn/tiêu đề/giải thích, KHÔNG mở lại dấu ``` nếu đang ở " +
  "giữa 1 khối code — chỉ xuất ra phần nội dung tiếp theo thuần tuý.";

/**
 * Tiếp tục 1 câu trả lời assistant bị cắt vì chạm giới hạn output token.
 *
 * Khác streamReply: KHÔNG gọi appendUserMessage — không tạo user turn mới
 * trong DB. Nếu chỉ ẩn bubble user ở FE mà vẫn lưu 2 message tách rời trong
 * Mongo, F5 lại trang sẽ thấy văn bản/code dài bị TÁCH ĐÔI VĨNH VIỄN. Thay
 * vào đó: CONTINUE_INSTRUCTION chỉ tồn tại TẠM THỜI trong request gửi agent
 * (nối vào cuối history, không persist), và response được NỐI vào cuối
 * assistant message cũ qua appendToLastAssistantMessage thay vì tạo message
 * mới — đúng 1 message liền mạch trong DB, giống hệt những gì user thấy.
 *
 * Không dùng chat cache (khác streamReply): cache key dựa trên history+model,
 * mà history ở đây luôn kết thúc bằng CONTINUE_INSTRUCTION cố định — cache
 * gần như không bao giờ hit thật (mỗi lượt continue là tiếp nối 1 ngữ cảnh
 * riêng biệt), thêm cache chỉ tăng phức tạp không lợi ích.
 */
export async function continueReply(
  conversationId: string,
  signal?: AbortSignal,
  agent: AgentClient = defaultAgent,
  tenantId?: string,
): Promise<StreamResult> {
  const raw = (await getMessagesRepo(
    conversationId,
  )) as unknown as ChatMessage[];

  if (raw.length === 0 || raw[raw.length - 1].role !== "assistant") {
    throw new BadRequestError(
      "Không có câu trả lời nào để tiếp tục — hội thoại trống hoặc tin nhắn cuối không phải của assistant.",
    );
  }

  const history = [
    ...toAnthropicMessages(raw),
    { role: "user" as const, content: CONTINUE_INSTRUCTION },
  ];

  let full = "";
  let tokensUsed = 0;
  let truncated = false;
  let backend: string = config.AGENT_BACKEND;
  let contextTokens: number | undefined;
  let contextBudget: number | undefined;

  let metadataResolve!: (meta: StreamMetadata) => void;
  const metadata = new Promise<StreamMetadata>((resolve) => {
    metadataResolve = resolve;
  });

  async function* generator(): AsyncGenerator<AgentEvent> {
    try {
      for await (const ev of agent.stream(history, { signal, tenantId })) {
        if (ev.type === "text") full += ev.text;

        if (ev.type === "truncated") truncated = true;

        if (ev.type === "done") {
          if (ev.agent) backend = ev.agent;
          if (ev.tokens) tokensUsed = ev.tokens;
          if (ev.truncated) truncated = true;
          if (ev.contextTokens !== undefined) contextTokens = ev.contextTokens;
          if (ev.contextBudget !== undefined) contextBudget = ev.contextBudget;
        }

        yield ev;
      }
    } finally {
      metadataResolve({
        backend,
        tokensUsed,
        truncated,
        contextTokens,
        contextBudget,
      });

      if (full.trim().length > 0) {
        await appendToLastAssistantMessage(conversationId, full);
      }
    }
  }

  return { events: generator(), metadata };
}
