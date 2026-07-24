import { useCallback, useState } from "react";
import Markdown from "./Markdown";
import {
  SparkIcon,
  CopyIcon,
  CheckIcon,
} from "@/shared/components/icons";
import AgentBadge from "@/shared/components/AgentBadge";
import ToolCallCard from "@/shared/components/ToolCallCard";
import CitationList from "@/shared/components/CitationList";
import type { Message } from "@/modules/chat/chat.api";
import type { ToolCallState, CitationData, UsageData } from "@/modules/chat/chat.api";

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
    return (
      <div className="flex justify-end animate-msg-in">
        <div className="max-w-[80%] whitespace-pre-wrap rounded-3xl rounded-br-lg bg-subtle px-4 py-2.5 text-[0.95rem] leading-relaxed text-ink">
          {message.content}
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
