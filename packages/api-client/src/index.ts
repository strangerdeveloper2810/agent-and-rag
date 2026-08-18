import { createHttpClient, HttpClient } from "@app/http";
import type {
  Conversation,
  AttachmentPayload,
  Message,
  ChatEvent,
  UsageData,
} from "@app/types";

export type * from "@app/types";

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

export type UploadResult =
  | ({ filename: string; ok: true } & DocumentInfo)
  | { filename: string; ok: false; error: string };

const defaultHttp = createHttpClient("/api");

// ----- Chat API Methods -----

export const createConversation = (
  firstMessage: string,
  client: HttpClient = defaultHttp,
) => client.post<Conversation>("/conversations", { firstMessage });

export const listConversations = (client: HttpClient = defaultHttp) =>
  client.get<Conversation[]>("/conversations");

export const getMessages = (id: string, client: HttpClient = defaultHttp) =>
  client.get<Message[]>(`/conversations/${id}/messages`);

export const deleteConversation = (
  id: string,
  client: HttpClient = defaultHttp,
) => client.delete<void>(`/conversations/${id}`);

export const renameConversation = (
  id: string,
  title: string,
  client: HttpClient = defaultHttp,
) => client.put<Conversation>(`/conversations/${id}`, { title });

/** Đọc 1 response SSE, parse từng dòng `data: {...}` → ChatEvent → onEvent.
 * Dùng chung cho streamChat và streamContinue (khác nhau ở URL/body gọi). */
const consumeSSEStream = async (
  response: Response,
  onEvent: (e: ChatEvent) => void,
): Promise<void> => {
  if (!response.body) throw new Error("No stream body received from server");

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  const parseLines = (lines: string[]) => {
    for (const line of lines) {
      const trimmed = line.trim();
      if (!trimmed.startsWith("data: ")) continue;
      try {
        const parsed = JSON.parse(trimmed.slice(6)) as Record<string, unknown>;
        const event = normalizeEvent(parsed);
        if (event) onEvent(event);
      } catch {
        // Ignore malformed SSE lines
      }
    }
  };

  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split("\n");
      buffer = lines.pop() ?? "";
      parseLines(lines);
    }
    buffer += decoder.decode();
    if (buffer.trim()) {
      parseLines(buffer.split("\n"));
    }
  } finally {
    reader.releaseLock();
  }
};

export const streamChat = async (
  conversationId: string,
  content: string,
  onEvent: (e: ChatEvent) => void,
  signal?: AbortSignal,
  attachments?: AttachmentPayload[],
  client: HttpClient = defaultHttp,
): Promise<void> => {
  const body: Record<string, unknown> = { content };
  if (attachments && attachments.length > 0) {
    body.attachments = attachments;
  }
  const response = await client.stream(
    `/conversations/${conversationId}/chat`,
    body,
    { signal },
  );
  await consumeSSEStream(response, onEvent);
};

/**
 * Tiếp tục 1 câu trả lời bị cắt vì chạm giới hạn output token — khác
 * streamChat: KHÔNG gửi content (không phải 1 user turn mới). Response được
 * BFF nối vào cuối message assistant cũ, nên FE cũng phải nối text stream về
 * vào ĐÚNG message đó thay vì tạo bubble mới (xem ChatPage.handleContinue).
 */
export const streamContinue = async (
  conversationId: string,
  onEvent: (e: ChatEvent) => void,
  signal?: AbortSignal,
  client: HttpClient = defaultHttp,
): Promise<void> => {
  const response = await client.stream(
    `/conversations/${conversationId}/continue`,
    undefined,
    { signal },
  );
  await consumeSSEStream(response, onEvent);
};

function normalizeEvent(raw: Record<string, unknown>): ChatEvent | null {
  const str = (k: string) =>
    (typeof raw[k] === "string" ? raw[k] : undefined) as string | undefined;
  const num = (k: string) =>
    (typeof raw[k] === "number" ? raw[k] : undefined) as number | undefined;
  const usage = isUsageData(raw.usage) ? raw.usage : undefined;
  // `tokens` là alias BFF/controller đang dùng cho tổng token (chat.controller
  // ghi {done:true, tokens}). Không nhận alias này thì đồng hồ token luôn
  // undefined dù backend tính đúng.
  const totalTokens = num("totalTokens") ?? num("tokens");
  const contextTokens = num("contextTokens");
  const contextBudget = num("contextBudget");
  const type = typeof raw.type === "string" ? raw.type : undefined;

  if (type === "step") return { type: "step", node: str("node") };
  if (type === "text") return { type: "text", text: str("text") ?? "" };
  if (type === "tool_start")
    return { type: "tool_start", name: str("name") ?? "unknown" };
  if (type === "tool_end")
    return {
      type: "tool_end",
      name: str("name") ?? "unknown",
      message: str("message"),
      text: str("text"),
    };
  if (type === "citation") return { type: "citation", text: str("text") };
  if (type === "memory") return { type: "memory", message: str("message") };
  if (type === "agent")
    return { type: "agent", name: str("name"), message: str("message") };
  if (type === "interrupt")
    return { type: "interrupt", name: str("name"), message: str("message") };
  if (type === "error") return { type: "error", message: str("message") };
  if (type === "usage") return { type: "usage", usage, totalTokens };
  if (type === "truncated")
    return { type: "truncated", message: str("message") };
  if (type === "done")
    return {
      type: "done",
      usage,
      totalTokens,
      truncated: raw.truncated === true,
      contextTokens,
      contextBudget,
    };

  // Legacy: done flag without explicit type
  if (raw.done === true)
    return {
      type: "done",
      usage,
      totalTokens,
      truncated: raw.truncated === true,
      contextTokens,
      contextBudget,
    };

  // Legacy: flat { token } → text event
  if (typeof raw.token === "string") return { type: "text", text: raw.token };

  return null;
}

function isUsageData(v: unknown): v is UsageData {
  if (!v || typeof v !== "object") return false;
  const u = v as Record<string, unknown>;
  return (
    typeof u.inputTokens === "number" && typeof u.outputTokens === "number"
  );
}

// ----- Documents API Methods -----

const fileForm = (file: File) => {
  const form = new FormData();
  form.append("file", file);
  return form;
};

export const listDocuments = (client: HttpClient = defaultHttp) =>
  client.get<DocumentInfo[]>("/documents");

export const uploadDocuments = (
  files: File[],
  client: HttpClient = defaultHttp,
) => {
  const form = new FormData();
  for (const f of files) form.append("file", f);
  return client.post<{ results: UploadResult[] }>("/documents/upload", form);
};

export const updateDocument = (
  documentId: string,
  file: File,
  client: HttpClient = defaultHttp,
) => client.put<DocumentInfo>(`/documents/${documentId}`, fileForm(file));

export const getVersions = (
  documentId: string,
  client: HttpClient = defaultHttp,
) => client.get<DocumentVersion[]>(`/documents/${documentId}/versions`);

export const getVersionContent = (
  documentId: string,
  version: number,
  client: HttpClient = defaultHttp,
) => client.get<VersionContent>(`/documents/${documentId}/versions/${version}`);

export const deleteDocument = (
  documentId: string,
  client: HttpClient = defaultHttp,
) => client.delete<void>(`/documents/${documentId}`);
