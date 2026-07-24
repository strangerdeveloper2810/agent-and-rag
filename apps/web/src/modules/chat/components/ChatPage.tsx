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

/** Extended message that carries UI-specific metadata from SSE events. */
export type MessageMeta = {
  /** Tool calls that happened during this assistant turn. */
  toolCalls: ToolCallState[];
  /** Citations (RAG sources) shown at the bottom of the message. */
  citations: CitationData[];
  /** Which agent responded (general/code/research). */
  agent: string | null;
  /** Token usage shown after completion. */
  usage: UsageData | null;
};

// ── Helpers ──

/** Read a File as raw base64 (no data URL prefix). */
function fileToBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => {
      const result = reader.result as string;
      // Strip "data:mime/type;base64," prefix
      const comma = result.indexOf(",");
      resolve(comma >= 0 ? result.slice(comma + 1) : result);
    };
    reader.onerror = () => reject(new Error(`Failed to read ${file.name}`));
    reader.readAsDataURL(file);
  });
}

/** Convert a pending attachment to the payload the API expects. */
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

/** Convert a pending attachment to display metadata for the message bubble. */
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

  // --- Smart auto-scroll ---
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

  // Reset scroll flag when conversation changes
  useEffect(() => {
    userScrolledUpRef.current = false;
    setTimeout(() => endRef.current?.scrollIntoView({ behavior: "auto", block: "end" }), 50);
  }, [id]);

  // --- Append text to assistant bubble ---
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

  // --- Update metadata for the last assistant message ---
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

  // --- Load messages for current conversation ---
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

  // Cleanup: abort stream on unmount
  useEffect(() => () => streamCtrlRef.current?.abort(), []);

  // Auto-scroll when messages change
  useEffect(() => {
    scrollToBottom(false, streaming);
  }, [messages, scrollToBottom, streaming]);

  // --- Send message ---
  const send = async (text?: string) => {
    const content = (text ?? input).trim();
    if ((!content && attachments.length === 0) || streaming) return;

    // Snapshot and clear immediately
    setInput("");
    const snapAttachments = [...attachments];
    setAttachments([]);

    // Build display metadata for the user message bubble
    const attachmentMeta: AttachmentMeta[] = snapAttachments.map(pendingToMeta);

    let convId = id;
    try {
      if (!convId) {
        // Use first attachment name or text for the conversation title
        const title = content || snapAttachments[0]?.name || "Tệp đính kèm";
        const conv = await createConversation(title);
        convId = conv._id;
        loadedIdRef.current = convId;
        navigate(`/messages/${convId}`);
        reloadConversations();
      }

      // Convert attachments to API payload (async reads files to base64)
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
                // Invalid citation JSON
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
              // Human-in-the-loop interrupt — toast handled by Composer/Toast
              break;

            case "error":
              setMessages((m) => {
                if (m.length > 0 && m[m.length - 1].role === "assistant") {
                  const copy = [...m];
                  copy[copy.length - 1] = {
                    ...copy[copy.length - 1],
                    content: copy[copy.length - 1].content +
                      `\n\n⚠️ ${e.message ?? "An error occurred."}`,
                  };
                  return copy;
                }
                return [...m, {
                  role: "assistant",
                  content: `⚠️ ${e.message ?? "An error occurred."}`,
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
                "\n\n⚠️ Could not send message. Please try again.",
            };
            return copy;
          }
          return [...m, {
            role: "assistant",
            content: "⚠️ Could not send message. Please try again.",
          }];
        });
        // Restore on error
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

  // --- Stop generation ---
  const stopGeneration = () => {
    streamCtrlRef.current?.abort();
  };

  const hasMessages = messages.length > 0;

  return (
    <main className="flex min-w-0 flex-1 flex-col" style={{ minHeight: 0 }}>
      {/* Streaming indicator bar */}
      {streaming && (
        <div className="flex shrink-0 items-center justify-between border-b border-line bg-gblue-soft/30 px-4 py-1.5 sm:px-6 dark:bg-gblue-soft/20">
          <span className="flex items-center gap-2 text-xs text-gblue">
            <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-gblue" />
            Generating response...
          </span>
          <button
            type="button"
            onClick={stopGeneration}
            aria-label="Stop generating"
            className="flex items-center gap-1 rounded-full px-3 py-1 text-xs font-medium text-ink-soft hover:bg-subtle transition"
          >
            <StopIcon width={14} height={14} />
            Stop
          </button>
        </div>
      )}

      <div
        ref={scrollContainerRef}
        onScroll={handleScroll}
        className="scroll-fine overflow-y-auto"
        style={{ flex: 1, minHeight: 0 }}
      >
        {hasMessages ? (
          <div className="mx-auto max-w-3xl space-y-7 px-4 py-6 sm:px-6">
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
