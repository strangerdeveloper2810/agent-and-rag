import { useState } from "react";
import Markdown from "./Markdown";
import { SparkIcon, CopyIcon, CheckIcon } from "./icons";
import type { Message } from "../lib/api";

/** Ba chấm nhảy khi assistant đang "suy nghĩ" (chưa có token nào). */
function TypingDots() {
  return (
    <span className="inline-flex items-center gap-1 py-1">
      {[0, 1, 2].map((i) => (
        <span
          key={i}
          className="h-1.5 w-1.5 rounded-full bg-ink-soft animate-dot-bounce"
          style={{ animationDelay: `${i * 0.16}s` }}
        />
      ))}
    </span>
  );
}

export default function MessageBubble({
  message,
  streaming = false,
}: {
  message: Message;
  /** true khi đây là bong bóng assistant cuối cùng đang stream dở */
  streaming?: boolean;
}) {
  const [copied, setCopied] = useState(false);
  const isUser = message.role === "user";

  const copy = async () => {
    await navigator.clipboard.writeText(message.content);
    setCopied(true);
    setTimeout(() => setCopied(false), 1600);
  };

  // ----- Tin nhắn người dùng: bong bóng accent, canh phải -----
  if (isUser) {
    return (
      <div className="flex justify-end animate-msg-in">
        <div className="max-w-[80%] whitespace-pre-wrap rounded-3xl rounded-br-lg bg-accent px-4 py-2.5 text-[0.95rem] leading-relaxed text-white shadow-bubble">
          {message.content}
        </div>
      </div>
    );
  }

  // ----- Tin nhắn assistant: avatar + nội dung markdown, canh trái -----
  return (
    <div className="group flex gap-3 animate-msg-in">
      <div className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-accent-glow to-accent-ink text-white shadow-bubble">
        <SparkIcon width={17} height={17} />
      </div>

      <div className="min-w-0 flex-1">
        {message.content ? (
          <Markdown content={message.content} />
        ) : streaming ? (
          <TypingDots />
        ) : null}

        {/* Con trỏ nhấp nháy khi đang stream */}
        {streaming && message.content && (
          <span className="ml-0.5 inline-block h-4 w-[2px] translate-y-0.5 bg-accent animate-caret-blink align-middle" />
        )}

        {/* Nút copy: chỉ hiện khi hover, ẩn lúc đang stream */}
        {!streaming && message.content && (
          <button
            type="button"
            onClick={copy}
            aria-label="Sao chép câu trả lời"
            className="mt-1.5 inline-flex items-center gap-1.5 rounded-lg px-2 py-1 text-xs text-ink-faint opacity-0 transition hover:bg-line/60 hover:text-ink-soft focus:opacity-100 group-hover:opacity-100"
          >
            {copied ? (
              <>
                <CheckIcon width={13} height={13} className="text-accent" />
                Đã chép
              </>
            ) : (
              <>
                <CopyIcon width={13} height={13} />
                Sao chép
              </>
            )}
          </button>
        )}
      </div>
    </div>
  );
}
