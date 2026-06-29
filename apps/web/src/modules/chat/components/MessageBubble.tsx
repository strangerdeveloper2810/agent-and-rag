import { useState } from "react";
import Markdown from "./Markdown";
import { SparkIcon, CopyIcon, CheckIcon } from "@/shared/components/icons";
import type { Message } from "@/modules/chat/chat.api";

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

export default function MessageBubble({
  message,
  streaming = false,
}: {
  message: Message;
  streaming?: boolean;
}) {
  const [copied, setCopied] = useState(false);
  const isUser = message.role === "user";

  const copy = async () => {
    await navigator.clipboard.writeText(message.content);
    setCopied(true);
    setTimeout(() => setCopied(false), 1600);
  };

  // Tin nhắn người dùng: bong bóng xám-xanh nhạt, canh phải (kiểu Gemini)
  if (isUser) {
    return (
      <div className="flex justify-end animate-msg-in">
        <div className="max-w-[80%] whitespace-pre-wrap rounded-3xl rounded-br-lg bg-subtle px-4 py-2.5 text-[0.95rem] leading-relaxed text-ink">
          {message.content}
        </div>
      </div>
    );
  }

  // Tin nhắn assistant: avatar sao Gemini + nội dung markdown, canh trái
  return (
    <div className="group flex gap-4 animate-msg-in">
      <div className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-gemini text-white">
        <SparkIcon width={17} height={17} />
      </div>

      <div className="min-w-0 flex-1">
        {message.content ? (
          <Markdown content={message.content} />
        ) : streaming ? (
          <TypingDots />
        ) : null}

        {streaming && message.content && (
          <span className="ml-0.5 inline-block h-4 w-[2px] translate-y-0.5 bg-gblue animate-caret-blink align-middle" />
        )}

        {!streaming && message.content && (
          <button
            type="button"
            onClick={copy}
            aria-label="Sao chép câu trả lời"
            className="mt-2 inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs text-ink-faint opacity-0 transition hover:bg-subtle focus:opacity-100 group-hover:opacity-100"
          >
            {copied ? (
              <>
                <CheckIcon width={13} height={13} className="text-gblue" />
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
