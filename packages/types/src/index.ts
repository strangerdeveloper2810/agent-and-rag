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

// ChatEvent — discriminated union matching Go agent event.go
export type ChatEvent =
  | { type: "step"; node?: string }
  | { type: "text"; text: string }
  | { type: "tool_start"; name: string }
  | { type: "tool_end"; name: string; message?: string }
  | { type: "citation"; text?: string }
  | { type: "memory"; message?: string }
  | { type: "agent"; name?: string; message?: string }
  | { type: "interrupt"; name?: string; message?: string }
  | { type: "error"; message?: string }
  | { type: "usage"; usage?: UsageData; totalTokens?: number }
  | { type: "done"; usage?: UsageData; totalTokens?: number };

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
