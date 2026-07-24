import { useCallback, useEffect, useRef, useState } from "react";
import { useNavigate, useOutletContext, useParams } from "react-router-dom";
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
import type { OutletCtx } from "@/shared/components/AppLayout";
import type { PendingAttachment } from "./Composer";
import MessageBubble from "./MessageBubble";
import Composer from "./Composer";
import EmptyState from "./EmptyState";
import { StopIcon } from "@/shared/components/icons";

export type MessageMeta = {
  toolCalls: ToolCallState[];
  citations: CitationData[];
  agent: string | null;
  usage: UsageData | null;
};

// ── Helpers ──

function fileToBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    // For large images, resize before base64 to avoid huge payloads
    if (file.type.startsWith("image/") && file.size > 1024 * 1024) {
      resizeImage(file, 1200).then(resolve).catch(() => {
        // Fallback: read original if resize fails
        const reader = new FileReader();
        reader.onload = () => {
          const result = reader.result as string;
          const comma = result.indexOf(",");
          resolve(comma >= 0 ? result.slice(comma + 1) : result);
        };
        reader.onerror = () => reject(new Error(`Failed to read ${file.name}`));
        reader.readAsDataURL(file);
      });
      return;
    }
    const reader = new FileReader();
    reader.onload = () => {
      const result = reader.result as string;
      const comma = result.indexOf(",");
      resolve(comma >= 0 ? result.slice(comma + 1) : result);
    };
    reader.onerror = () => reject(new Error(`Failed to read ${file.name}`));
    reader.readAsDataURL(file);
  });
}

function resizeImage(file: File, maxWidth: number): Promise<string> {
  return new Promise((resolve, reject) => {
    const img = new Image();
    const url = URL.createObjectURL(file);
    img.onload = () => {
      URL.revokeObjectURL(url);
      const scale = Math.min(1, maxWidth / img.width);
      if (scale >= 1) {
        // Image is already small enough, read as-is
        const reader = new FileReader();
        reader.onload = () => {
          const result = reader.result as string;
          const comma = result.indexOf(",");
          resolve(comma >= 0 ? result.slice(comma + 1) : result);
        };
        reader.onerror = () => reject(new Error("Failed to read"));
        reader.readAsDataURL(file);
        return;
      }
      const canvas = document.createElement("canvas");
      canvas.width = Math.round(img.width * scale);
      canvas.height = Math.round(img.height * scale);
      const ctx = canvas.getContext("2d")!;
      ctx.drawImage(img, 0, 0, canvas.width, canvas.height);
      const dataUrl = canvas.toDataURL("image/jpeg", 0.85);
      const comma = dataUrl.indexOf(",");
      resolve(comma >= 0 ? dataUrl.slice(comma + 1) : dataUrl);
    };
    img.onerror = () => {
      URL.revokeObjectURL(url);
      reject(new Error("Failed to load image"));
    };
    img.src = url;
  });
}

async function pendingToPayload(
  pa: PendingAttachment,
): Promise<AttachmentPayload> {
  const data = await fileToBase64(pa.file);
  return {
    type: pa.type,
    name: pa.name,
    data,
    mimeType: pa.file.type || (pa.type === "image" ? "image/png" : "application/octet-stream"),
    size: pa.size,
  };
}

function pendingToMeta(pa: PendingAttachment): AttachmentMeta {
  return {
    type: pa.type,
    name: pa.name,
    size: pa.size,
    mimeType: pa.file.type || (pa.type === "image" ? "image/png" : "application/octet-stream"),
    thumbnail: pa.type === "image" ? pa.preview : "",
  };
}

export default function ChatPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const { reloadConversations } = useOutletContext<OutletCtx>();

  const [messages, setMessages] = useState<Message[]>([]);
  const [meta, setMeta] = useState<Map<number, MessageMeta>>(new Map());
  const [input, setInput] = useState("");
  const [attachments, setAttachments] = useState<PendingAttachment[]>([]);
  const [streaming, setStreaming] = useState(false);
  const endRef = useRef<HTMLDivElement>(null);
  const scrollContainerRef = useRef<HTMLDivElement>(null);
  const loadedIdRef = useRef<string | null>(null);
  const streamCtrlRef = useRef<AbortController | null>(null);
  const userScrolledUpRef = useRef(false);

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
    setTimeout(() => endRef.current?.scrollIntoView({ behavior: "auto", block: "end" }), 50);
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
        };
        next.set(index, updater(current));
        return next;
      });
    },
    [],
  );

  useEffect(() => {
    if (!id) {
      setMessages([]);
      setMeta(new Map());
      loadedIdRef.current = null;
      return;
    }
    if (id === loadedIdRef.current) return;

    streamCtrlRef.current?.abort();
    setStreaming(false);
    loadedIdRef.current = id;

    getMessages(id)
      .then((m) => {
        if (loadedIdRef.current === id) setMessages(m);
      })
      .catch(() => {
        if (loadedIdRef.current === id) setMessages([]);
      });
  }, [id]);

  useEffect(() => () => streamCtrlRef.current?.abort(), []);

  useEffect(() => {
    scrollToBottom(false, streaming);
  }, [messages, scrollToBottom, streaming]);

  const send = async (text?: string) => {
    const content = (text ?? input).trim();
    if ((!content && attachments.length === 0) || streaming) return;

    setInput("");
    const snapAttachments = [...attachments];
    setAttachments([]);

    const attachmentMeta: AttachmentMeta[] = snapAttachments.map(pendingToMeta);

    let convId = id;
    try {
      if (!convId) {
        const title = content || snapAttachments[0]?.name || "Attachment";
        const conv = await createConversation(title);
        convId = conv._id;
        loadedIdRef.current = convId;
        navigate(`/messages/${convId}`);
        reloadConversations();
      }

      const attachmentPayloads: AttachmentPayload[] = await Promise.all(
        snapAttachments.map(pendingToPayload),
      );

      const assistantIndex = messages.length + 1;
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
                    result: e.message ? undefined : "Completed",
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
              break;
            case "error":
              setMessages((m) => {
                if (m.length > 0 && m[m.length - 1].role === "assistant") {
                  const copy = [...m];
                  copy[copy.length - 1] = {
                    ...copy[copy.length - 1],
                    content: copy[copy.length - 1].content +
                      `\n\n⚠ ${e.message ?? "An error occurred."}`,
                  };
                  return copy;
                }
                return [...m, {
                  role: "assistant",
                  content: `⚠ ${e.message ?? "An error occurred."}`,
                }];
              });
              userScrolledUpRef.current = false;
              break;
            case "done":
              if (e.usage) {
                updateMeta(assistantIndex, (prev) => ({
                  ...prev,
                  usage: e.usage ?? null,
                }));
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
              content: copy[copy.length - 1].content +
                "\n\n⚠ Could not send message. Please try again.",
            };
            return copy;
          }
          return [...m, {
            role: "assistant",
            content: "⚠ Could not send message. Please try again.",
          }];
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
    }
  };

  const stopGeneration = () => {
    streamCtrlRef.current?.abort();
  };

  const hasMessages = messages.length > 0;

  return (
    <main className="flex min-w-0 flex-1 flex-col" style={{ minHeight: 0 }}>
      {/* Streaming indicator */}
      {streaming && (
        <div
          className="flex shrink-0 items-center justify-between px-4 py-1.5 sm:px-6"
          style={{
            borderBottom: "1px solid var(--cyber-border)",
            background: "linear-gradient(90deg, rgba(0,240,255,0.05) 0%, transparent 50%, rgba(0,240,255,0.05) 100%)",
          }}
        >
          <span className="flex items-center gap-2 text-[11px]" style={{ color: "var(--cyber-primary)" }}>
            <span
              className="h-1.5 w-1.5 animate-pulse rounded-full"
              style={{ backgroundColor: "var(--cyber-primary)" }}
            />
            Processing...
          </span>
          <button
            type="button"
            onClick={stopGeneration}
            aria-label="Stop generating"
            className="flex items-center gap-1 rounded-full px-3 py-1 text-[11px] font-medium transition hover:bg-[var(--cyber-subtle)]"
            style={{ color: "var(--cyber-muted)" }}
          >
            <StopIcon width={12} height={12} />
            Stop
          </button>
        </div>
      )}

      <div
        ref={scrollContainerRef}
        onScroll={handleScroll}
        className="scroll-fine overflow-y-auto"
        style={{ flex: 1, minHeight: 0, backgroundColor: "var(--cyber-bg)" }}
      >
        {hasMessages ? (
          <div className="mx-auto max-w-3xl space-y-6 px-4 py-6 sm:px-6">
            {messages.map((m, i) => {
              const msgMeta = meta.get(i);
              const isLastAssistant =
                streaming && i === messages.length - 1 && m.role === "assistant";
              return (
                <MessageBubble
                  key={m._id ?? i}
                  message={m}
                  streaming={isLastAssistant}
                  toolCalls={msgMeta?.toolCalls ?? []}
                  citations={msgMeta?.citations ?? []}
                  agent={msgMeta?.agent ?? null}
                  usage={msgMeta?.usage ?? null}
                />
              );
            })}
            <div ref={endRef} />
          </div>
        ) : (
          <EmptyState onPick={(p) => send(p)} />
        )}
      </div>

      <div className="shrink-0">
        <Composer
          value={input}
          onChange={setInput}
          onSend={() => send()}
          disabled={streaming}
          attachments={attachments}
          onAttachmentsChange={setAttachments}
        />
      </div>
    </main>
  );
}
