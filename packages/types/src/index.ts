// Conversation & Message Types
export interface Conversation {
  _id: string;
  title: string;
  createdAt?: string;
  updatedAt?: string;
}

export interface AttachmentPayload {
  type: "image" | "file";
  name: string;
  data: string; // base64 string
  mimeType: string;
  size?: number;
}

export interface AttachmentMeta {
  type: "image" | "file";
  name: string;
  size: number;
  mimeType: string;
  thumbnail?: string;
}

export interface Message {
  _id?: string;
  role: "user" | "assistant";
  content: string;
  attachments?: AttachmentMeta[];
  createdAt?: string;
}

// SSE & Telemetry Event Types
export interface UsageData {
  inputTokens: number;
  outputTokens: number;
}

export interface CitationData {
  title: string;
  url?: string;
  snippet?: string;
}

export interface ToolCallState {
  name: string;
  status: "running" | "done" | "error";
  result?: string;
  error?: string;
}

export interface ChatEvent {
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
  node?: string;
  text?: string;
  name?: string;
  message?: string;
  usage?: UsageData;
}

// Task & Document Types
export interface Task {
  _id?: string;
  title: string;
  status: "pending" | "in_progress" | "completed";
  priority?: "low" | "medium" | "high";
  tags?: string[];
  dueDate?: string;
  remindAt?: string;
  source?: string;
  createdAt?: string;
  updatedAt?: string;
}

export interface DocumentChunk {
  _id?: string;
  documentId: string;
  source: string;
  version: number;
  chunkIndex: number;
  text: string;
  createdAt?: string;
}
