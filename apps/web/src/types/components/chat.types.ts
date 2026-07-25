import type { Message, ToolCallState, CitationData, UsageData } from "@app/types";

/** Metadata structure tracked per message. */
export type MessageMeta = {
  toolCalls: ToolCallState[];
  citations: CitationData[];
  agent: string | null;
  usage: UsageData | null;
};

/** Props for PendingAttachment preview items in Composer. */
export type PendingAttachment = {
  id: string;
  file: File;
  type: "image" | "file";
  preview: string;
  name: string;
  size: number;
};

/** Props for Composer input organism component. */
export interface ComposerProps {
  onSend: (text: string, attachments: PendingAttachment[]) => void;
  disabled?: boolean;
  streaming?: boolean;
  onStop?: () => void;
}

/** Props for EmptyState centerpiece component. */
export interface EmptyStateProps {
  onPick: (prompt: string) => void;
}

/** Props for MessageBubble component. */
export interface MessageBubbleProps {
  message: Message;
  meta?: MessageMeta;
}
