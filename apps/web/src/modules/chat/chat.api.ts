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

export const renameConversation = (id: string, title: string) =>
  http.put<Conversation>(`/conversations/${id}`, { title });

// --- SSE Event Types (Go agent → frontend) ---

/** Token usage reported on "done" event. */
export type UsageData = {
  inputTokens: number;
  outputTokens: number;
};

/** Citation from RAG search. */
export type CitationData = {
  title: string;
  url?: string;
  snippet?: string;
};

/** Tool execution state tracked across tool_start → tool_end. */
export type ToolCallState = {
  name: string;
  status: "running" | "done" | "error";
  result?: string;
  error?: string;
};

/** One SSE event from the Go agent engine.
 *  Type list: step | text | tool_start | tool_end | citation | memory | agent | interrupt | error | done
 *  The engine emits these in order; the UI assembles them into a coherent response. */
export type ChatEvent = {
  type:
    | "step"
    | "text"
    | "tool_start"
    | "tool_end"
    | "citation"
    | "memory"
    | "agent"
    | "interrupt"
    | "error"
    | "done";
  /** step: current node id (recall, summarize, model, tools, extract) */
  node?: string;
  /** text: streaming token; citation: JSON array of CitationData */
  text?: string;
  /** tool_start / tool_end / interrupt: tool name */
  name?: string;
  /** error / memory / interrupt: detail message */
  message?: string;
  /** done: accumulated token usage */
  usage?: UsageData;
};

// --- Streaming ---

/**
 * Send a message and stream SSE events from the Go agent.
 * Each event is parsed and passed to `onEvent` as it arrives.
 *
 * @param conversationId - The conversation to send to
 * @param content - User message text
 * @param onEvent - Callback for each parsed SSE event
 * @param signal - AbortSignal to cancel the stream
 */
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
          const parsed = JSON.parse(line.slice(6)) as ChatEvent;
          // Normalize: if server sends flat {token, type, name, message, done}
          // map them into the structured ChatEvent shape.
          const event = normalizeEvent(parsed);
          if (event) onEvent(event);
        } catch {
          // Ignore malformed / keep-alive SSE lines
        }
      }
    }
    // Flush any remaining multi-byte characters
    buffer += decoder.decode();
  } finally {
    reader.releaseLock();
  }
};

/**
 * Normalize incoming SSE data into a well-formed ChatEvent.
 * Handles both the old flat format and the new structured format.
 */
function normalizeEvent(raw: Record<string, unknown>): ChatEvent | null {
  // Already has a valid 'type' field in the new structure
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

  // Legacy flat format: { token?, type?, name?, message?, done? }
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
          typeof raw.message === "string"
            ? raw.message
            : "An error occurred",
      };
    }
    if (raw.type === "text" && typeof raw.text === "string") {
      return { type: "text", text: raw.text };
    }
  }
  // Oldest format: { token? } → treat as text
  if (typeof raw.token === "string") {
    return { type: "text", text: raw.token };
  }

  return null;
}

function isUsageData(v: unknown): v is UsageData {
  if (!v || typeof v !== "object") return false;
  const u = v as Record<string, unknown>;
  return typeof u.inputTokens === "number" && typeof u.outputTokens === "number";
}
