import { useCallback, useState } from "react";
import Markdown from "./Markdown";
import {
  SparkIcon,
  CopyIcon,
  CheckIcon,
  CloseIcon,
  DocIcon,
} from "@/shared/components/icons";
import AgentBadge from "@/shared/components/AgentBadge";
import ToolCallCard from "@/shared/components/ToolCallCard";
import CitationList from "@/shared/components/CitationList";
import type { Message, AttachmentMeta } from "@/modules/chat/chat.api";
import type { ToolCallState, CitationData, UsageData } from "@/modules/chat/chat.api";

// ── Helpers ──

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

// ── Sub-components ──

/** Animated typing dots shown when the assistant is streaming but hasn't produced text yet. */
function TypingDots() {
  return (
    <span className="inline-flex items-center gap-1 py-1">
      {[0, 1, 2].map((i) => (
        <span
          key={i}
          className="h-2 w-2 rounded-full bg-gblue animate-dot-bounce"
          style={{ animationDelay: `${i * 0.16}s` }}
        />
      ))}
    </span>
  );
}

/** Token usage display */
function UsageFooter({ usage }: { usage: UsageData }) {
  return (
    <div className="mt-3 border-t border-line pt-2">
      <p className="text-xs text-ink-faint">
        <span title="Input tokens">{usage.inputTokens.toLocaleString()} in</span>
        {" · "}
        <span title="Output tokens">{usage.outputTokens.toLocaleString()} out</span>
        {" · "}
        <span title="Total tokens">
          {(usage.inputTokens + usage.outputTokens).toLocaleString()} total
        </span>
      </p>
    </div>
  );
}

/** Attachment thumbnails shown in user message bubbles. */
function AttachmentList({ attachments }: { attachments: AttachmentMeta[] }) {
  const [expandedUrl, setExpandedUrl] = useState<string | null>(null);

  const images = attachments.filter((a) => a.type === "image");
  const files = attachments.filter((a) => a.type === "file");

  return (
    <>
      <div className="mt-2 space-y-2">
        {/* Image grid */}
        {images.length > 0 && (
          <div className="flex flex-wrap gap-1.5">
            {images.map((img, i) => (
              <button
                key={`img-${i}`}
                type="button"
                onClick={() => setExpandedUrl(img.thumbnail)}
                aria-label={`Xem ảnh ${img.name}`}
                className="h-16 w-16 overflow-hidden rounded-xl border border-line bg-subtle transition hover:opacity-80"
              >
                <img
                  src={img.thumbnail}
                  alt={img.name}
                  className="h-full w-full object-cover"
                />
              </button>
            ))}
          </div>
        )}

        {/* File chips */}
        {files.length > 0 && (
          <div className="space-y-1">
            {files.map((f, i) => (
              <div
                key={`file-${i}`}
                className="flex items-center gap-2 rounded-lg border border-line bg-subtle px-3 py-1.5"
              >
                <DocIcon width={15} height={15} className="shrink-0 text-ink-faint" />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-[0.82rem] font-medium text-ink">
                    {f.name}
                  </p>
                  <p className="text-[11px] text-ink-faint">
                    {formatSize(f.size)}
                  </p>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Lightbox overlay */}
      {expandedUrl && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4 animate-fade-in"
          onClick={() => setExpandedUrl(null)}
          role="dialog"
          aria-label="Xem ảnh phóng to"
        >
          <button
            type="button"
            onClick={() => setExpandedUrl(null)}
            aria-label="Đóng ảnh"
            className="absolute right-4 top-4 flex h-9 w-9 items-center justify-center rounded-full bg-white/20 text-white transition hover:bg-white/30"
          >
            <CloseIcon width={18} height={18} />
          </button>
          <img
            src={expandedUrl}
            alt="Phóng to"
            className="max-h-[85vh] max-w-[90vw] rounded-2xl object-contain shadow-2xl"
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

export default function MessageBubble({
  message,
  streaming = false,
  toolCalls = [],
  citations = [],
  agent = null,
  usage = null,
}: MessageBubbleProps) {
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
      <div className="flex justify-end animate-msg-in">
        <div className="max-w-[80%] rounded-3xl rounded-br-lg bg-subtle px-4 py-2.5 text-[0.95rem] leading-relaxed text-ink">
          {hasText && (
            <p className="whitespace-pre-wrap">{message.content}</p>
          )}
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
    <div className="group flex gap-4 animate-msg-in">
      {/* Avatar */}
      <div className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-gemini text-white">
        <SparkIcon width={17} height={17} />
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
          <div className="mb-2 space-y-1">
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
          <span className="ml-0.5 inline-block h-4 w-[2px] translate-y-0.5 animate-caret-blink bg-gblue align-middle" />
        )}

        {/* Citations */}
        {citations.length > 0 && !streaming && (
          <CitationList citations={citations} />
        )}

        {/* Token usage */}
        {usage && !streaming && <UsageFooter usage={usage} />}

        {/* Copy button (visible on hover for completed messages) */}
        {!streaming && hasContent && (
          <button
            type="button"
            onClick={copyMessage}
            aria-label="Copy response"
            className="mt-2 inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs text-ink-faint opacity-0 transition hover:bg-subtle focus:opacity-100 group-hover:opacity-100"
          >
            {copied ? (
              <>
                <CheckIcon width={13} height={13} className="text-gblue" />
                Copied
              </>
            ) : (
              <>
                <CopyIcon width={13} height={13} />
                Copy
              </>
            )}
          </button>
        )}
      </div>
    </div>
  );
}
