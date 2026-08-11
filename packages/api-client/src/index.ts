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

export const createConversation = (firstMessage: string, client: HttpClient = defaultHttp) =>
  client.post<Conversation>("/conversations", { firstMessage });

export const listConversations = (client: HttpClient = defaultHttp) =>
  client.get<Conversation[]>("/conversations");

export const getMessages = (id: string, client: HttpClient = defaultHttp) =>
  client.get<Message[]>(`/conversations/${id}/messages`);

export const deleteConversation = (id: string, client: HttpClient = defaultHttp) =>
  client.delete<void>(`/conversations/${id}`);

export const renameConversation = (id: string, title: string, client: HttpClient = defaultHttp) =>
  client.put<Conversation>(`/conversations/${id}`, { title });

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
  if (!response.body) throw new Error("No stream body received from server");

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
        try {
          const parsed = JSON.parse(line.slice(6)) as Record<string, unknown>;
          const event = normalizeEvent(parsed);
          if (event) onEvent(event);
        } catch {
          // Ignore malformed SSE lines
        }
      }
    }
    buffer += decoder.decode();
  } finally {
    reader.releaseLock();
  }
};

function normalizeEvent(raw: Record<string, unknown>): ChatEvent | null {
  const validTypes = [
    "step",
    "text",
    "tool_start",
    "tool_end",
    "citation",
    "memory",
    "agent",
    "interrupt",
    "error",
    "done",
  ];
  if (typeof raw.type === "string" && validTypes.includes(raw.type)) {
    return {
      type: raw.type as ChatEvent["type"],
      node: typeof raw.node === "string" ? raw.node : undefined,
      text: typeof raw.text === "string" ? raw.text : undefined,
      name: typeof raw.name === "string" ? raw.name : undefined,
      message: typeof raw.message === "string" ? raw.message : undefined,
      usage: isUsageData(raw.usage) ? raw.usage : undefined,
    };
  }

  if (raw.done === true) {
    return {
      type: "done",
      usage: isUsageData(raw.usage) ? raw.usage : undefined,
    };
  }
  if (typeof raw.type === "string") {
    if (raw.type === "tool_start") {
      return {
        type: "tool_start",
        name: typeof raw.name === "string" ? raw.name : "unknown",
      };
    }
    if (raw.type === "tool_end") {
      return {
        type: "tool_end",
        name: typeof raw.name === "string" ? raw.name : "unknown",
        message: typeof raw.message === "string" ? raw.message : undefined,
      };
    }
    if (raw.type === "error") {
      return {
        type: "error",
        message:
          typeof raw.message === "string" ? raw.message : "An error occurred",
      };
    }
    if (raw.type === "text" && typeof raw.text === "string") {
      return { type: "text", text: raw.text };
    }
  }
  if (typeof raw.token === "string") {
    return { type: "text", text: raw.token };
  }

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

export const uploadDocuments = (files: File[], client: HttpClient = defaultHttp) => {
  const form = new FormData();
  for (const f of files) form.append("file", f);
  return client.post<{ results: UploadResult[] }>("/documents/upload", form);
};

export const updateDocument = (documentId: string, file: File, client: HttpClient = defaultHttp) =>
  client.put<DocumentInfo>(`/documents/${documentId}`, fileForm(file));

export const getVersions = (documentId: string, client: HttpClient = defaultHttp) =>
  client.get<DocumentVersion[]>(`/documents/${documentId}/versions`);

export const getVersionContent = (documentId: string, version: number, client: HttpClient = defaultHttp) =>
  client.get<VersionContent>(`/documents/${documentId}/versions/${version}`);

export const deleteDocument = (documentId: string, client: HttpClient = defaultHttp) =>
  client.delete<void>(`/documents/${documentId}`);
