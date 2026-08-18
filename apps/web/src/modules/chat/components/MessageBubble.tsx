import { useCallback, useState } from "react";
import {
  ClipboardDocumentIcon,
  CheckIcon,
  SpeakerWaveIcon,
  SpeakerXMarkIcon,
  ArrowPathIcon,
  HandThumbUpIcon,
  HandThumbDownIcon,
  CpuChipIcon,
  DocumentTextIcon,
  XMarkIcon,
  ExclamationTriangleIcon,
} from "@heroicons/react/24/outline";

import Markdown from "./Markdown";
import AgentBadge from "@/design-system/atoms/AgentBadge";
import { ToolCallGroup } from "@/design-system/molecules/ToolCallCard";
import CitationList from "@/design-system/molecules/CitationList";
import type { Message, AttachmentMeta } from "@/modules/chat/chat.api";
import type {
  ToolCallState,
  CitationData,
  UsageData,
} from "@/modules/chat/chat.api";

import { Button } from "@/components/ui/button";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";

// ── Helpers ──

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function TypingDots() {
  return (
    <span className="inline-flex items-center gap-1.5 py-1.5 px-1">
      {[0, 1, 2].map((i) => (
        <span
          key={i}
          className="h-2 w-2 rounded-full bg-primary animate-bounce"
          style={{
            animationDelay: `${i * 0.16}s`,
          }}
        />
      ))}
    </span>
  );
}

function UsageFooter({ usage }: { usage: UsageData }) {
  return (
    <div className="mt-3 border-t border-border pt-2 flex items-center gap-3 text-[10px] text-muted-foreground font-mono">
      <span>Input: {usage.inputTokens.toLocaleString()} tokens</span>
      <span>·</span>
      <span>Output: {usage.outputTokens.toLocaleString()} tokens</span>
      <span>·</span>
      <span className="text-primary font-semibold">
        Tổng: {(usage.inputTokens + usage.outputTokens).toLocaleString()} tokens
      </span>
    </div>
  );
}

/**
 * AttachmentList component for rendering image previews and file chips attached to messages.
 */
const AttachmentList: React.FC<{ attachments: AttachmentMeta[] }> = ({
  attachments,
}) => {
  const [expandedUrl, setExpandedUrl] = useState<string | null>(null);

  const images = attachments.filter((a) => a.type === "image");
  const files = attachments.filter((a) => a.type === "file");

  return (
    <>
      <div className="mt-2.5 space-y-2">
        {images.length > 0 && (
          <div className="flex flex-wrap gap-2">
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
                  aria-label={`Xem ${img.name}`}
                  className="h-20 w-20 overflow-hidden rounded-2xl border border-border bg-card transition hover:opacity-80 shadow-sm"
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
          <div className="space-y-1.5">
            {files.map((f, i) => (
              <div
                key={`file-${i}`}
                className="flex items-center gap-2.5 rounded-xl border border-border bg-card px-3 py-2"
              >
                <DocumentTextIcon className="h-4 w-4 text-primary" />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-xs font-semibold text-foreground">
                    {f.name}
                  </p>
                  <p className="text-[10px] text-muted-foreground font-mono">
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
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 p-4 animate-fade-in backdrop-blur-sm"
          onClick={() => setExpandedUrl(null)}
        >
          <Button
            type="button"
            variant="ghost"
            size="icon"
            onClick={() => setExpandedUrl(null)}
            aria-label="Đóng ảnh"
            className="absolute right-4 top-4 text-white hover:bg-white/20 rounded-full"
          >
            <XMarkIcon className="h-5 w-5" />
          </Button>
          <img
            src={expandedUrl}
            alt="Enlarged"
            className="max-h-[85vh] max-w-[90vw] rounded-2xl object-contain shadow-2xl"
            onClick={(e) => e.stopPropagation()}
          />
        </div>
      )}
    </>
  );
};

// ── Main Component ──

interface MessageBubbleProps {
  message: Message;
  streaming?: boolean;
  toolCalls?: ToolCallState[];
  citations?: CitationData[];
  agent?: string | null;
  usage?: UsageData | null;
  /** Câu trả lời bị cắt vì chạm giới hạn độ dài → hiện chỉ báo + nút "Tiếp tục". */
  truncated?: boolean;
  /** Quá trình xử lý gặp lỗi thực sự từ stream/mạng. */
  hasError?: boolean;
  onRegenerate?: () => void;
  onRetryUser?: (content: string) => void;
  onContinue?: () => void;
}

/**
 * MessageBubble component — ChatGPT & Gemini style with Retry / Run Again actions.
 */
export const MessageBubble: React.FC<MessageBubbleProps> = ({
  message,
  streaming = false,
  toolCalls = [],
  citations = [],
  agent = null,
  usage = null,
  truncated = false,
  hasError = false,
  onRegenerate,
  onRetryUser,
  onContinue,
}) => {
  const [copied, setCopied] = useState(false);
  const [isSpeaking, setIsSpeaking] = useState(false);
  const [feedback, setFeedback] = useState<"up" | "down" | null>(null);

  const isUser = message.role === "user";

  const copyMessage = useCallback(async () => {
    await navigator.clipboard.writeText(message.content);
    setCopied(true);
    setTimeout(() => setCopied(false), 1600);
  }, [message.content]);

  const toggleSpeech = useCallback(() => {
    if (!("speechSynthesis" in window)) return;
    if (isSpeaking) {
      window.speechSynthesis.cancel();
      setIsSpeaking(false);
    } else {
      window.speechSynthesis.cancel();
      const utterance = new SpeechSynthesisUtterance(message.content);
      utterance.lang = "vi-VN";
      utterance.onend = () => setIsSpeaking(false);
      utterance.onerror = () => setIsSpeaking(false);
      setIsSpeaking(true);
      window.speechSynthesis.speak(utterance);
    }
  }, [isSpeaking, message.content]);

  // --- User bubble (with hover Retry action) ---
  if (isUser) {
    const hasAttachments =
      message.attachments && message.attachments.length > 0;
    const hasText = message.content.length > 0;

    return (
      <div className="group/user flex items-center justify-end gap-2 animate-fade-in my-1">
        {onRetryUser && hasText && (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => onRetryUser(message.content)}
            aria-label="Thử lại tin nhắn này"
            title="Gửi lại yêu cầu này cho Agent"
            className="opacity-0 group-hover/user:opacity-100 transition-opacity duration-150 h-8 px-2.5 text-xs text-muted-foreground hover:text-primary hover:bg-primary/10 gap-1.5 rounded-xl border border-transparent hover:border-primary/20"
          >
            <ArrowPathIcon className="h-3.5 w-3.5" />
            <span>Thử lại</span>
          </Button>
        )}
        <div className="max-w-[85%] sm:max-w-[75%] rounded-[22px] rounded-tr-md px-5 py-3.5 text-sm sm:text-base leading-relaxed bg-[#f4f4f5] dark:bg-[#27272a] text-[#09090b] dark:text-[#f4f4f5] shadow-xs">
          {hasText && (
            <p className="whitespace-pre-wrap font-sans tracking-normal">
              {message.content}
            </p>
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
  const isErrorMessage =
    hasError ||
    (!streaming &&
      message.role === "assistant" &&
      (message.content.includes("⚠ Could not send message") ||
        message.content === "AI agent không phản hồi" ||
        message.content === "AI agent tạm thời không khả dụng"));
  const showTyping = streaming && !hasContent && toolCalls.length === 0;

  return (
    <div className="group flex gap-3.5 animate-fade-in">
      {/* Bot Avatar */}
      <Avatar className="h-9 w-9 mt-0.5 border border-primary/30 shadow-md relative shrink-0">
        <AvatarFallback className="bg-gradient-to-br from-indigo-500 to-purple-600 text-white">
          <CpuChipIcon className="h-5 w-5" />
        </AvatarFallback>
        {streaming && (
          <span className="absolute -top-0.5 -right-0.5 h-2.5 w-2.5 rounded-full bg-primary animate-ping" />
        )}
      </Avatar>

      <div className="min-w-0 flex-1">
        {/* Agent Identity Badge */}
        {agent && (
          <div className="mb-2">
            <AgentBadge agent={agent} />
          </div>
        )}

        {/* Tool Call Group */}
        {toolCalls.length > 0 && <ToolCallGroup tools={toolCalls} />}

        {/* Message Content */}
        {hasContent ? (
          <div className="prose-slate dark:prose-invert relative">
            <Markdown content={message.content} />
            {streaming && (
              <span className="inline-block w-2 h-4.5 bg-primary ml-1 animate-pulse align-middle" />
            )}
          </div>
        ) : showTyping ? (
          <TypingDots />
        ) : null}

        {/* Error Banner with Prominent Retry Action */}
        {isErrorMessage && !streaming && (
          <div className="mt-3.5 flex flex-wrap sm:flex-nowrap items-center justify-between gap-3 rounded-2xl border border-destructive/30 bg-destructive/10 p-3.5 text-xs text-destructive backdrop-blur-md">
            <div className="flex items-center gap-2.5 font-semibold min-w-0">
              <ExclamationTriangleIcon className="h-5 w-5 shrink-0 text-destructive" />
              <span className="leading-snug">
                Phản hồi gặp sự cố hoặc quá trình xử lý bị gián đoạn.
              </span>
            </div>
            {onRegenerate && (
              <Button
                type="button"
                variant="destructive"
                size="sm"
                onClick={onRegenerate}
                className="gap-1.5 font-bold shadow-sm shrink-0"
              >
                <ArrowPathIcon className="h-3.5 w-3.5" />
                <span>Thử lại ngay</span>
              </Button>
            )}
          </div>
        )}

        {/* Active Streaming Indicator when LLM is thinking/processing tool calls */}
        {streaming && (hasContent || toolCalls.length > 0) && (
          <div className="mt-3 flex items-center gap-2 rounded-xl bg-primary/10 border border-primary/20 px-3.5 py-2 text-xs text-primary shadow-xs">
            <span className="relative flex h-2 w-2">
              <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-primary opacity-75" />
              <span className="relative inline-flex rounded-full h-2 w-2 bg-primary" />
            </span>
            <span className="font-semibold tracking-wide">
              J.A.R.V.I.S. đang suy luận & xử lý dữ liệu...
            </span>
            <span className="inline-flex gap-1 ml-1">
              <span
                className="h-1.5 w-1.5 rounded-full bg-primary animate-bounce"
                style={{ animationDelay: "0s" }}
              />
              <span
                className="h-1.5 w-1.5 rounded-full bg-primary animate-bounce"
                style={{ animationDelay: "0.15s" }}
              />
              <span
                className="h-1.5 w-1.5 rounded-full bg-primary animate-bounce"
                style={{ animationDelay: "0.3s" }}
              />
            </span>
          </div>
        )}

        {/* Truncated Banner — câu trả lời chạm giới hạn độ dài */}
        {truncated && !streaming && (
          <div
            role="status"
            className="mt-3.5 flex flex-wrap sm:flex-nowrap items-center justify-between gap-3 rounded-2xl border border-amber-500/30 bg-amber-500/10 p-3.5 text-xs text-amber-700 dark:text-amber-400 backdrop-blur-md"
          >
            <div className="flex items-center gap-2.5 font-semibold min-w-0">
              <ExclamationTriangleIcon className="h-5 w-5 shrink-0" />
              <span className="leading-snug">
                Câu trả lời bị cắt do chạm giới hạn độ dài tối đa.
              </span>
            </div>
            {onContinue && (
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={onContinue}
                aria-label="Yêu cầu agent viết tiếp câu trả lời"
                title="Yêu cầu agent viết tiếp từ chỗ bị cắt"
                className="gap-1.5 font-bold shadow-sm shrink-0 border-amber-500/40 hover:bg-amber-500/20"
              >
                <ArrowPathIcon className="h-3.5 w-3.5" />
                <span>Tiếp tục</span>
              </Button>
            )}
          </div>
        )}

        {/* Citations */}
        {citations.length > 0 && !streaming && (
          <CitationList citations={citations} />
        )}

        {/* Token Usage Footer */}
        {usage && !streaming && <UsageFooter usage={usage} />}

        {/* Smart Action Bar */}
        {!streaming && hasContent && (
          <div className="mt-3 flex flex-wrap items-center gap-1.5 opacity-80 group-hover:opacity-100 transition duration-150">
            {/* Copy button */}
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={copyMessage}
              aria-label="Sao chép nội dung"
              title="Sao chép câu trả lời"
              className="gap-1 h-7 text-[11px]"
            >
              {copied ? (
                <>
                  <CheckIcon className="h-3.5 w-3.5 text-emerald-400" />
                  <span className="text-emerald-400">Đã chép</span>
                </>
              ) : (
                <>
                  <ClipboardDocumentIcon className="h-3.5 w-3.5" />
                  <span>Sao chép</span>
                </>
              )}
            </Button>

            {/* Read Aloud (TTS) */}
            <Button
              type="button"
              variant={isSpeaking ? "secondary" : "outline"}
              size="sm"
              onClick={toggleSpeech}
              aria-label="Đọc câu trả lời"
              title="Đọc văn bản bằng giọng nói"
              className={`gap-1 h-7 text-[11px] ${isSpeaking ? "border-primary text-primary animate-pulse" : ""}`}
            >
              {isSpeaking ? (
                <SpeakerXMarkIcon className="h-3.5 w-3.5" />
              ) : (
                <SpeakerWaveIcon className="h-3.5 w-3.5" />
              )}
              <span>{isSpeaking ? "Dừng đọc" : "Đọc tiếng"}</span>
            </Button>

            {/* Regenerate button */}
            {onRegenerate && (
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={onRegenerate}
                aria-label="Tạo lại câu trả lời"
                title="Yêu cầu AI trả lời lại"
                className="gap-1 h-7 text-[11px]"
              >
                <ArrowPathIcon className="h-3.5 w-3.5" />
                <span>Tạo lại</span>
              </Button>
            )}

            {/* Thumbs Up / Down Feedback */}
            <div className="ml-1 flex items-center gap-1 border-l border-border pl-2">
              <Button
                type="button"
                variant="ghost"
                size="iconSm"
                onClick={() => setFeedback((f) => (f === "up" ? null : "up"))}
                aria-label="Hài lòng"
                title="Đánh giá tốt"
                className={`h-7 w-7 ${feedback === "up" ? "text-emerald-400 bg-emerald-500/10" : "text-muted-foreground"}`}
              >
                <HandThumbUpIcon className="h-3.5 w-3.5" />
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="iconSm"
                onClick={() =>
                  setFeedback((f) => (f === "down" ? null : "down"))
                }
                aria-label="Chưa hài lòng"
                title="Đánh giá cần cải thiện"
                className={`h-7 w-7 ${feedback === "down" ? "text-rose-400 bg-rose-500/10" : "text-muted-foreground"}`}
              >
                <HandThumbDownIcon className="h-3.5 w-3.5" />
              </Button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};

export default MessageBubble;
