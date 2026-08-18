import { useCallback, useEffect, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { SparklesIcon, XMarkIcon } from "@heroicons/react/24/outline";
import {
  createConversation,
  getMessages,
  streamChat,
  type Message,
  type ChatEvent,
  type ToolCallState,
  type CitationData,
  type UsageData,
  type AttachmentPayload,
  type AttachmentMeta,
} from "@/modules/chat/chat.api";
import type { PendingAttachment } from "./Composer";
import MessageBubble from "./MessageBubble";
import Composer from "./Composer";
import EmptyState from "./EmptyState";
import ChatSkeleton from "@/design-system/molecules/ChatSkeleton";
import { useToast } from "@/design-system/molecules/Toast";
import { validateComposerInput } from "@/lib/validation";
import { useDocumentTitle } from "@/hooks/useDocumentTitle";
import { Button } from "@/components/ui/button";

export type MessageMeta = {
  toolCalls: ToolCallState[];
  citations: CitationData[];
  agent: string | null;
  usage: UsageData | null;
  /** true khi câu trả lời bị cắt vì chạm giới hạn output token. */
  truncated: boolean;
};

/** Prompt gửi khi user bấm "Tiếp tục" trên một câu trả lời bị cắt. */
const CONTINUE_PROMPT = "Tiếp tục câu trả lời từ chỗ bị cắt.";

/** Trạng thái gợi ý bắt đầu chat mới khi context đã lớn (Tier 4). */
export type ContextWarning = { tokens: number; budget: number };

/** Tỉ lệ contextTokens/contextBudget để bắt đầu gợi ý chat mới. */
const CONTEXT_WARNING_RATIO = 0.8;

// ── Helpers ──

import type {
  FileToBase64Fn,
  OptimizeImageFn,
  PendingToPayloadFn,
  PendingToMetaFn,
} from "@/types";

const MAX_IMAGE_WIDTH = 800;
const MAX_IMAGE_HEIGHT = 800;
const JPEG_QUALITY = 0.75;

const fileToBase64: FileToBase64Fn = (file: File): Promise<string> => {
  return new Promise((resolve, reject) => {
    if (file.type.startsWith("image/") && file.type !== "image/svg+xml") {
      optimizeImage(file)
        .then(resolve)
        .catch(() => fallbackRead(file).then(resolve).catch(reject));
      return;
    }
    fallbackRead(file).then(resolve).catch(reject);
  });
};

const fallbackRead = (file: File): Promise<string> => {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => {
      const result = reader.result as string;
      const comma = result.indexOf(",");
      resolve(comma >= 0 ? result.slice(comma + 1) : result);
    };
    reader.onerror = () => reject(new Error(`Failed to read ${file.name}`));
    reader.readAsDataURL(file);
  });
};

const optimizeImage: OptimizeImageFn = (file: File): Promise<string> => {
  return new Promise((resolve, reject) => {
    const img = new Image();
    const url = URL.createObjectURL(file);
    img.onload = () => {
      URL.revokeObjectURL(url);
      let w = img.width;
      let h = img.height;
      if (w > MAX_IMAGE_WIDTH || h > MAX_IMAGE_HEIGHT) {
        const scale = Math.min(MAX_IMAGE_WIDTH / w, MAX_IMAGE_HEIGHT / h);
        w = Math.round(w * scale);
        h = Math.round(h * scale);
      }

      const canvas = document.createElement("canvas");
      canvas.width = w;
      canvas.height = h;
      const ctx = canvas.getContext("2d")!;
      ctx.fillStyle = "#ffffff";
      ctx.fillRect(0, 0, w, h);
      ctx.drawImage(img, 0, 0, w, h);

      const dataUrl = canvas.toDataURL("image/jpeg", JPEG_QUALITY);
      const comma = dataUrl.indexOf(",");
      const base64 = comma >= 0 ? dataUrl.slice(comma + 1) : dataUrl;

      resolve(base64);
    };
    img.onerror = () => {
      URL.revokeObjectURL(url);
      reject(new Error("Failed to load image"));
    };
    img.src = url;
  });
};

const pendingToPayload: PendingToPayloadFn = async (
  pa: PendingAttachment,
): Promise<AttachmentPayload> => {
  const data = await fileToBase64(pa.file);
  return {
    type: pa.type,
    name: pa.name,
    data,
    mimeType:
      pa.type === "image"
        ? "image/jpeg"
        : pa.file.type || "application/octet-stream",
    size: pa.size,
  };
};

/**
 * Converts client-side PendingAttachment to display AttachmentMeta.
 *
 * @param pa - PendingAttachment object
 * @returns AttachmentMeta display object
 */
const pendingToMeta: PendingToMetaFn = (
  pa: PendingAttachment,
): AttachmentMeta => {
  return {
    type: pa.type,
    name: pa.name,
    size: pa.size,
    mimeType:
      pa.file.type ||
      (pa.type === "image" ? "image/png" : "application/octet-stream"),
    thumbnail: pa.type === "image" ? pa.preview : "",
  };
};

import { useConversation } from "@/context/ConversationContext";

/**
 * ChatPage component for rendering active conversation and streaming SSE responses.
 */
export const ChatPage: React.FC = () => {
  const { id } = useParams();
  const navigate = useNavigate();
  const { reloadConversations } = useConversation();

  const [messages, setMessages] = useState<Message[]>([]);
  const [meta, setMeta] = useState<Map<number, MessageMeta>>(new Map());
  const [input, setInput] = useState("");
  const [attachments, setAttachments] = useState<PendingAttachment[]>([]);
  const [streaming, setStreaming] = useState(false);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [contextWarning, setContextWarning] = useState<ContextWarning | null>(
    null,
  );
  const endRef = useRef<HTMLDivElement>(null);
  const scrollContainerRef = useRef<HTMLDivElement>(null);
  const loadedIdRef = useRef<string | null>(null);
  const isCreatingNewConvRef = useRef<string | null>(null);
  const streamCtrlRef = useRef<AbortController | null>(null);
  const userScrolledUpRef = useRef(false);
  const toast = useToast();
  useDocumentTitle("Chat");

  const streamingRef = useRef(false);
  streamingRef.current = streaming;

  const scrollToBottom = useCallback((force = false, instant = false) => {
    if (!force && userScrolledUpRef.current) return;
    endRef.current?.scrollIntoView({
      behavior: instant ? "auto" : "smooth",
      block: "end",
    });
  }, []);

  const handleScroll = useCallback(() => {
    const el = scrollContainerRef.current;
    if (!el) return;
    const threshold = streamingRef.current ? 200 : 120;
    userScrolledUpRef.current =
      el.scrollHeight - el.scrollTop - el.clientHeight > threshold;
  }, []);

  useEffect(() => {
    userScrolledUpRef.current = false;
    setTimeout(
      () => endRef.current?.scrollIntoView({ behavior: "auto", block: "end" }),
      50,
    );
  }, [id]);

  const appendToAssistant = useCallback(
    (list: Message[], text: string): Message[] => {
      if (list.length === 0) return list;
      const last = list[list.length - 1];
      if (last.role !== "assistant") return list;
      const copy = [...list];
      copy[copy.length - 1] = { ...last, content: last.content + text };
      return copy;
    },
    [],
  );

  const updateMeta = useCallback(
    (index: number, updater: (prev: MessageMeta) => MessageMeta) => {
      setMeta((prev) => {
        const next = new Map(prev);
        const current = next.get(index) ?? {
          toolCalls: [],
          citations: [],
          agent: null,
          usage: null,
          truncated: false,
        };
        next.set(index, updater(current));
        return next;
      });
    },
    [],
  );

  useEffect(() => {
    if (!id) {
      if (!isCreatingNewConvRef.current) {
        setMessages([]);
        setMeta(new Map());
        loadedIdRef.current = null;
        setHistoryLoading(false);
      }
      return;
    }
    if (id === loadedIdRef.current || id === isCreatingNewConvRef.current) {
      loadedIdRef.current = id;
      isCreatingNewConvRef.current = null;
      return;
    }

    streamCtrlRef.current?.abort();
    setStreaming(false);
    loadedIdRef.current = id;
    setHistoryLoading(true);
    setMessages([]);
    setMeta(new Map());

    getMessages(id)
      .then((m) => {
        if (loadedIdRef.current === id) setMessages(m);
      })
      .catch(() => {
        if (loadedIdRef.current === id) setMessages([]);
      })
      .finally(() => {
        if (loadedIdRef.current === id) setHistoryLoading(false);
      });
  }, [id]);

  useEffect(() => () => streamCtrlRef.current?.abort(), []);

  useEffect(() => {
    scrollToBottom(false, streaming);
  }, [messages, scrollToBottom, streaming]);

  const send = async (text?: string) => {
    const rawContent = (text ?? input).trim();
    if ((!rawContent && attachments.length === 0) || streaming) return;

    if (rawContent) {
      const validation = validateComposerInput(rawContent);
      if (!validation.valid) {
        toast.error(validation.error);
        return;
      }
    }

    const content = rawContent;
    setInput("");
    const snapAttachments = [...attachments];
    setAttachments([]);

    const attachmentMeta: AttachmentMeta[] = snapAttachments.map(pendingToMeta);

    let convId = id;
    // Khai báo ngoài try để block finally dọn được tool còn treo "running".
    const assistantIndex = messages.length + 1;
    try {
      if (!convId) {
        const title = content || snapAttachments[0]?.name || "Attachment";
        const conv = await createConversation(title);
        convId = conv._id;
        isCreatingNewConvRef.current = convId;
        loadedIdRef.current = convId;
        navigate(`/messages/${convId}`);
        reloadConversations();
      }

      if (!convId) return;

      const attachmentPayloads: AttachmentPayload[] = await Promise.all(
        snapAttachments.map(pendingToPayload),
      );

      setMessages((m) => [
        ...m,
        {
          role: "user",
          content,
          attachments: attachmentMeta,
        },
        { role: "assistant", content: "" },
      ]);
      setStreaming(true);

      const ctrl = new AbortController();
      streamCtrlRef.current = ctrl;

      await streamChat(
        convId,
        content,
        (e: ChatEvent) => {
          if (loadedIdRef.current !== convId) return;

          switch (e.type) {
            case "step":
              break;
            case "text":
              setMessages((m) => appendToAssistant(m, e.text ?? ""));
              break;
            case "tool_start":
              updateMeta(assistantIndex, (prev) => ({
                ...prev,
                toolCalls: [
                  ...prev.toolCalls,
                  { name: e.name ?? "unknown", status: "running" },
                ],
              }));
              break;
            case "tool_end":
              updateMeta(assistantIndex, (prev) => {
                const tools = [...prev.toolCalls];
                let idx = -1;
                for (let i = tools.length - 1; i >= 0; i--) {
                  if (
                    tools[i].name === e.name &&
                    tools[i].status === "running"
                  ) {
                    idx = i;
                    break;
                  }
                }
                if (idx >= 0) {
                  tools[idx] = {
                    ...tools[idx],
                    status: e.message ? "error" : "done",
                    result: e.message ? undefined : (e.text ?? "Completed"),
                    error: e.message,
                  };
                }
                return { ...prev, toolCalls: tools };
              });
              break;
            case "citation":
              try {
                const citations: CitationData[] = e.text
                  ? JSON.parse(e.text)
                  : [];
                updateMeta(assistantIndex, (prev) => ({
                  ...prev,
                  citations,
                }));
              } catch {
                // ignore
              }
              break;
            case "memory":
              break;
            case "agent":
              updateMeta(assistantIndex, (prev) => ({
                ...prev,
                agent: e.name ?? null,
              }));
              break;
            case "interrupt":
              // Guardrails chặn một tool destructive (vd shell.exec). Phần giải
              // thích chi tiết do Go gửi qua event text nên đã nằm trong nội
              // dung câu trả lời; ở đây chỉ báo nổi cho user biết vì sao agent
              // dừng giữa đường. Trước đây case này rỗng nên user không có
              // manh mối nào.
              toast.warning(
                `Đã dừng: công cụ "${e.name ?? "không rõ"}" cần được xác nhận trước khi chạy.`,
              );
              break;
            case "error":
              setMessages((m) => {
                if (m.length > 0 && m[m.length - 1].role === "assistant") {
                  const copy = [...m];
                  copy[copy.length - 1] = {
                    ...copy[copy.length - 1],
                    content: copy[copy.length - 1].content,
                  };
                  return copy;
                }
                return m;
              });
              toast.error(
                "Đang có sự cố với dịch vụ AI, chúng tôi sẽ khắc phục trong giây lát. Vui lòng thử lại.",
              );
              userScrolledUpRef.current = false;
              break;
            case "usage":
              // Go phát usage sau MỖI lần gọi LLM, với Usage là số của RIÊNG
              // bước đó → cộng dồn để ra tổng cả lượt. Trước đây FE không hề
              // xử lý event này (và BFF cũng drop nó), nên UsageFooter không
              // bao giờ có số dù backend tính đúng.
              if (e.usage) {
                updateMeta(assistantIndex, (prev) => ({
                  ...prev,
                  usage: {
                    inputTokens:
                      (prev.usage?.inputTokens ?? 0) + e.usage!.inputTokens,
                    outputTokens:
                      (prev.usage?.outputTokens ?? 0) + e.usage!.outputTokens,
                  },
                }));
              }
              break;
            case "truncated":
              updateMeta(assistantIndex, (prev) => ({
                ...prev,
                truncated: true,
              }));
              break;
            case "done":
              if (e.usage || e.truncated) {
                updateMeta(assistantIndex, (prev) => ({
                  ...prev,
                  usage: e.usage ?? prev.usage,
                  truncated: prev.truncated || e.truncated === true,
                }));
              }
              // Context lớn (Tier 4): Go gửi kích thước context ước tính ở
              // cuối lượt + ngân sách qua event done. contextBudget=0/undefined
              // nghĩa là MAX_CONTEXT_TOKENS chưa cấu hình (không giới hạn) —
              // bỏ qua gợi ý trong trường hợp đó thay vì hiểu nhầm đã đầy.
              if (
                e.contextBudget &&
                e.contextBudget > 0 &&
                e.contextTokens !== undefined
              ) {
                setContextWarning(
                  e.contextTokens / e.contextBudget >= CONTEXT_WARNING_RATIO
                    ? { tokens: e.contextTokens, budget: e.contextBudget }
                    : null,
                );
              }
              break;
          }
        },
        ctrl.signal,
        attachmentPayloads.length > 0 ? attachmentPayloads : undefined,
      );
    } catch (err) {
      if ((err as Error)?.name !== "AbortError") {
        setMessages((m) => {
          if (m.length > 0 && m[m.length - 1].role === "assistant") {
            const copy = [...m];
            copy[copy.length - 1] = {
              ...copy[copy.length - 1],
              content:
                copy[copy.length - 1].content +
                "\n\n⚠ Could not send message. Please try again.",
            };
            return copy;
          }
          return [
            ...m,
            {
              role: "assistant",
              content: "⚠ Could not send message. Please try again.",
            },
          ];
        });
        setInput((prev) => prev || content);
        if (snapAttachments.length > 0) {
          setAttachments(snapAttachments);
        }
        userScrolledUpRef.current = false;
      }
    } finally {
      streamCtrlRef.current = null;
      setStreaming(false);
      // Dọn tool còn treo ở trạng thái "running". Stream đã kết thúc (xong,
      // lỗi, hoặc user bấm Stop) nên không còn event tool_end nào tới nữa —
      // nếu không dọn, card tool hiển thị "Đang thực thi N công cụ..." kèm
      // spinner VĨNH VIỄN dù đã dừng từ lâu.
      updateMeta(assistantIndex, (prev) => {
        if (!prev.toolCalls.some((t) => t.status === "running")) return prev;
        return {
          ...prev,
          toolCalls: prev.toolCalls.map((t) =>
            t.status === "running"
              ? { ...t, status: "error", error: "Đã dừng trước khi hoàn tất" }
              : t,
          ),
        };
      });
    }
  };

  const stopGeneration = () => {
    streamCtrlRef.current?.abort();
  };

  const handleRegenerate = useCallback(
    (index: number) => {
      if (streaming) return;
      let userText = "";
      for (let i = index; i >= 0; i--) {
        if (messages[i]?.role === "user") {
          userText = messages[i].content;
          break;
        }
      }
      if (userText) {
        send(userText);
      }
    },
    [messages, streaming],
  );

  const handleRetryUser = useCallback(
    (userContent: string) => {
      if (streaming) return;
      send(userContent);
    },
    [streaming],
  );

  // Câu trả lời bị cắt vì chạm giới hạn output token → gửi prompt yêu cầu
  // agent viết tiếp (phần đã nhận vẫn nằm trong history nên agent có ngữ cảnh).
  const handleContinue = useCallback(() => {
    if (streaming) return;
    send(CONTINUE_PROMPT);
  }, [streaming]);

  const hasMessages = messages.length > 0;

  return (
    <main className="flex min-w-0 flex-1 flex-col" style={{ minHeight: 0 }}>
      <div
        ref={scrollContainerRef}
        onScroll={handleScroll}
        className="scroll-fine overflow-y-auto"
        style={{ flex: 1, minHeight: 0, backgroundColor: "var(--bg)" }}
      >
        {historyLoading ? (
          <ChatSkeleton />
        ) : hasMessages ? (
          <div className="mx-auto max-w-3xl space-y-6 px-4 py-6 sm:px-6">
            {messages.map((m, i) => {
              const msgMeta = meta.get(i);
              const isLastAssistant =
                streaming &&
                i === messages.length - 1 &&
                m.role === "assistant";
              return (
                <MessageBubble
                  key={m._id ?? i}
                  message={m}
                  streaming={isLastAssistant}
                  toolCalls={msgMeta?.toolCalls ?? []}
                  citations={msgMeta?.citations ?? []}
                  agent={msgMeta?.agent ?? null}
                  usage={msgMeta?.usage ?? null}
                  truncated={msgMeta?.truncated ?? false}
                  onRegenerate={() => handleRegenerate(i)}
                  onRetryUser={(c) => handleRetryUser(c)}
                  onContinue={handleContinue}
                />
              );
            })}
            <div ref={endRef} />
          </div>
        ) : (
          <EmptyState onPick={(p) => send(p)} />
        )}
      </div>

      {contextWarning && (
        <div className="mx-auto w-full max-w-3xl shrink-0 px-4 pb-2 sm:px-6">
          <div
            role="status"
            className="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-indigo-500/30 bg-indigo-500/10 p-3.5 text-xs text-indigo-700 backdrop-blur-md sm:flex-nowrap dark:text-indigo-400"
          >
            <div className="flex min-w-0 items-center gap-2.5 font-semibold">
              <SparklesIcon className="h-5 w-5 shrink-0" />
              <span className="leading-snug">
                Cuộc trò chuyện đã khá dài (
                {Math.round(contextWarning.tokens / 1000)}K token) — bắt đầu
                chat mới để có phản hồi nhanh và chính xác hơn. Các thông tin
                quan trọng đã học vẫn được giữ lại.
              </span>
            </div>
            <div className="flex shrink-0 items-center gap-1.5">
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => navigate("/")}
                aria-label="Bắt đầu cuộc trò chuyện mới"
                title="Bắt đầu cuộc trò chuyện mới"
                className="gap-1.5 font-bold shadow-sm border-indigo-500/40 hover:bg-indigo-500/20"
              >
                <SparklesIcon className="h-3.5 w-3.5" />
                <span>Bắt đầu chat mới</span>
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => setContextWarning(null)}
                aria-label="Đóng gợi ý"
                title="Đóng"
              >
                <XMarkIcon className="h-3.5 w-3.5" />
              </Button>
            </div>
          </div>
        </div>
      )}

      <div className="shrink-0">
        <Composer
          value={input}
          onChange={setInput}
          onSend={() => send()}
          disabled={streaming}
          onStop={stopGeneration}
          attachments={attachments}
          onAttachmentsChange={setAttachments}
        />
      </div>
    </main>
  );
};

export default ChatPage;
