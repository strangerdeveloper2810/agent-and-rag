import { useCallback, useState } from "react";
import Markdown from "./Markdown";
import {
  CopyIcon,
  CheckIcon,
  CloseIcon,
  DocIcon,
} from "@/shared/components/icons";
import AgentBadge from "@/shared/components/AgentBadge";
import ToolCallCard from "@/shared/components/ToolCallCard";
import CitationList from "@/shared/components/CitationList";
import type { Message, AttachmentMeta } from "@/modules/chat/chat.api";
import type {
  ToolCallState,
  CitationData,
  UsageData,
} from "@/modules/chat/chat.api";

// ── Helpers ──

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

// ── Sub-components ──

function TypingDots() {
  return (
    <span className="inline-flex items-center gap-1 py-1">
      {[0, 1, 2].map((i) => (
        <span
          key={i}
          className="h-2 w-2 rounded-full animate-dot-bounce"
          style={{
            backgroundColor: "var(--accent)",
            animationDelay: `${i * 0.16}s`,
          }}
        />
      ))}
    </span>
  );
}

function UsageFooter({ usage }: { usage: UsageData }) {
  return (
    <div
      className="mt-3 border-t pt-2"
      style={{ borderColor: "var(--border)" }}
    >
      <p className="text-[10px]" style={{ color: "var(--text-tertiary)" }}>
        <span title="Input tokens">
          {usage.inputTokens.toLocaleString()} in
        </span>
        {" · "}
        <span title="Output tokens">
          {usage.outputTokens.toLocaleString()} out
        </span>
        {" · "}
        <span title="Total tokens">
          {(usage.inputTokens + usage.outputTokens).toLocaleString()} total
        </span>
      </p>
    </div>
  );
}

/**
 * AttachmentList component for rendering image previews and file chips attached to messages.
 */
const AttachmentList: React.FC<{ attachments: AttachmentMeta[] }> = ({ attachments }) => {
  const [expandedUrl, setExpandedUrl] = useState<string | null>(null);

  const images = attachments.filter((a) => a.type === "image");
  const files = attachments.filter((a) => a.type === "file");

  return (
    <>
      <div className="mt-2 space-y-2">
        {images.length > 0 && (
          <div className="flex flex-wrap gap-1.5">
            {images.map((img, i) => {
              const anyImg = img as any;
              const imgSrc =
                img.thumbnail ||
                anyImg.url ||
                (anyImg.data
                  ? anyImg.data.startsWith("data:")
                    ? anyImg.data
                    : `data:${img.mimeType || "image/jpeg"};base64,${anyImg.data}`
                  : "");

              if (!imgSrc) return null;

              return (
                <button
                  key={`img-${i}`}
                  type="button"
                  onClick={() => setExpandedUrl(imgSrc)}
                  aria-label={`View ${img.name}`}
                  className="h-16 w-16 overflow-hidden rounded-xl border transition hover:opacity-80 shadow-sm"
                  style={{
                    borderColor: "var(--border)",
                    backgroundColor: "var(--bg-raised)",
                  }}
                >
                  <img
                    src={imgSrc}
                    alt={img.name}
                    className="h-full w-full object-cover"
                  />
                </button>
              );
            })}
          </div>
        )}

        {files.length > 0 && (
          <div className="space-y-1">
            {files.map((f, i) => (
              <div
                key={`file-${i}`}
                className="flex items-center gap-2 rounded-lg border px-3 py-1.5"
                style={{
                  borderColor: "var(--border)",
                  backgroundColor: "var(--bg-raised)",
                }}
              >
                <DocIcon
                  width={14}
                  height={14}
                  style={{ color: "var(--text-tertiary)" }}
                />
                <div className="min-w-0 flex-1">
                  <p
                    className="truncate text-[11px] font-medium"
                    style={{ color: "var(--text)" }}
                  >
                    {f.name}
                  </p>
                  <p
                    className="text-[10px]"
                    style={{ color: "var(--text-tertiary)" }}
                  >
                    {formatSize(f.size)}
                  </p>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {expandedUrl && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 p-4 animate-fade-in"
          onClick={() => setExpandedUrl(null)}
          role="dialog"
          aria-label="View enlarged image"
        >
          <button
            type="button"
            onClick={() => setExpandedUrl(null)}
            aria-label="Close image"
            className="absolute right-4 top-4 flex h-9 w-9 items-center justify-center rounded-full bg-white/10 text-white transition hover:bg-white/20"
          >
            <CloseIcon width={18} height={18} />
          </button>
          <img
            src={expandedUrl}
            alt="Enlarged"
            className="max-h-[85vh] max-w-[90vw] rounded-xl object-contain shadow-2xl"
            onClick={(e) => e.stopPropagation()}
          />
        </div>
      )}
    </>
  );
}

// ── Main Component ──

interface MessageBubbleProps {
  message: Message;
  streaming?: boolean;
  toolCalls?: ToolCallState[];
  citations?: CitationData[];
  agent?: string | null;
  usage?: UsageData | null;
}

/**
 * MessageBubble component for rendering user chat bubbles & assistant responses
 * with streaming indicators, markdown formatting, tool executions, and citations.
 */
export const MessageBubble: React.FC<MessageBubbleProps> = ({
  message,
  streaming = false,
  toolCalls = [],
  citations = [],
  agent = null,
  usage = null,
}) => {
  const [copied, setCopied] = useState(false);
  const isUser = message.role === "user";

  const copyMessage = useCallback(async () => {
    await navigator.clipboard.writeText(message.content);
    setCopied(true);
    setTimeout(() => setCopied(false), 1600);
  }, [message.content]);

  // --- User bubble ---
  if (isUser) {
    const hasAttachments =
      message.attachments && message.attachments.length > 0;
    const hasText = message.content.length > 0;

    return (
      <div className="flex justify-end animate-fade-in">
        <div
          className="max-w-[85%] sm:max-w-[78%] rounded-2xl rounded-tr-xs px-4 py-3 text-xs sm:text-sm leading-relaxed border shadow-sm"
          style={{
            backgroundColor: "var(--user-bubble-bg)",
            color: "var(--user-bubble-text)",
            borderColor: "var(--user-bubble-border)",
          }}
        >
          {hasText && <p className="whitespace-pre-wrap font-sans">{message.content}</p>}
          {hasAttachments && (
            <AttachmentList attachments={message.attachments!} />
          )}
        </div>
      </div>
    );
  }

  // --- Assistant bubble ---
  const hasContent = message.content.length > 0;
  const showTyping = streaming && !hasContent && toolCalls.length === 0;
  const showCaret = streaming && hasContent;

  return (
    <div className="group flex gap-3.5 animate-fade-in">
      {/* Avatar */}
      <div
        className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-xl border shadow-sm"
        style={{
          borderColor: "var(--border)",
          backgroundColor: "var(--surface)",
        }}
      >
        <svg
          width={16}
          height={16}
          viewBox="0 0 24 24"
          fill="none"
          stroke="var(--accent)"
          strokeWidth={1.8}
          strokeLinecap="round"
          strokeLinejoin="round"
        >
          <path d="M12 3.5c.6 3.7 1.8 4.9 5.5 5.5-3.7.6-4.9 1.8-5.5 5.5-.6-3.7-1.8-4.9-5.5-5.5 3.7-.6 4.9-1.8 5.5-5.5Z" />
        </svg>
      </div>

      <div className="min-w-0 flex-1">
        {/* Agent badge */}
        {agent && (
          <div className="mb-2">
            <AgentBadge agent={agent} />
          </div>
        )}

        {/* Tool call cards */}
        {toolCalls.length > 0 && (
          <div className="mb-3 space-y-1.5">
            {toolCalls.map((tc, i) => (
              <ToolCallCard key={`${tc.name}-${i}`} tool={tc} />
            ))}
          </div>
        )}

        {/* Message content */}
        {hasContent ? (
          <Markdown content={message.content} />
        ) : showTyping ? (
          <TypingDots />
        ) : null}

        {/* Streaming caret */}
        {showCaret && (
          <span
            className="ml-0.5 inline-block h-[1em] w-[2px] translate-y-0.5 align-middle animate-terminal-blink"
            style={{ backgroundColor: "var(--accent)" }}
          />
        )}

        {/* Citations */}
        {citations.length > 0 && !streaming && (
          <CitationList citations={citations} />
        )}

        {/* Token usage */}
        {usage && !streaming && <UsageFooter usage={usage} />}

        {/* Copy button */}
        {!streaming && hasContent && (
          <button
            type="button"
            onClick={copyMessage}
            aria-label="Copy response"
            className="mt-2.5 inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-[10px] font-medium opacity-0 transition duration-150 hover:bg-[var(--bg-raised)] hover:border-[var(--accent)] focus:opacity-100 group-hover:opacity-100"
            style={{ 
              color: "var(--text-tertiary)",
              borderColor: "var(--border)",
            }}
          >
            {copied ? (
              <>
                <CheckIcon
                  width={12}
                  height={12}
                  style={{ color: "var(--success)" }}
                />
                Copied
              </>
            ) : (
              <>
                <CopyIcon width={12} height={12} />
                Copy message
              </>
            )}
          </button>
        )}
      </div>
    </div>
  );
};

export default MessageBubble;
