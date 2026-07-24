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
} from "@/modules/chat/chat.api";
import type { OutletCtx } from "@/shared/components/AppLayout";
import MessageBubble from "./MessageBubble";
import Composer from "./Composer";
import EmptyState from "./EmptyState";
import { StopIcon } from "@/shared/components/icons";
import { useToast } from "@/shared/components/Toast";

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

export default function ChatPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const { reloadConversations } = useOutletContext<OutletCtx>();
  const toast = useToast();

  const [messages, setMessages] = useState<Message[]>([]);
  const [meta, setMeta] = useState<Map<number, MessageMeta>>(new Map());
  const [input, setInput] = useState("");
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
    // During streaming, auto-scroll unless user explicitly scrolled up >200px
    const threshold = streamingRef.current ? 200 : 120;
    userScrolledUpRef.current =
      el.scrollHeight - el.scrollTop - el.clientHeight > threshold;
  }, []);

  // Reset scroll flag when conversation changes
  useEffect(() => {
    userScrolledUpRef.current = false;
    // Force instant scroll to bottom on conversation load
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

    // Abort any active stream when switching conversations
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

  // Auto-scroll when messages change — instant during streaming, smooth after
  useEffect(() => {
    scrollToBottom(false, streaming);
  }, [messages, scrollToBottom, streaming]);

  // --- Send message ---
  const send = async (text?: string) => {
    const content = (text ?? input).trim();
    if (!content || streaming) return;
    setInput("");

    let convId = id;
    try {
      if (!convId) {
        const conv = await createConversation(content);
        convId = conv._id;
        loadedIdRef.current = convId;
        navigate(`/messages/${convId}`);
        reloadConversations();
      }

      const assistantIndex = messages.length + 1; // 0 = user, 1 = assistant
      setMessages((m) => [
        ...m,
        { role: "user", content },
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
              // Engine step -- could show in debug UI
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
                // Find the last running tool call with matching name
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
              // Memory operations -- could show a small indicator
              break;

            case "agent":
              updateMeta(assistantIndex, (prev) => ({
                ...prev,
                agent: e.name ?? null,
              }));
              break;

            case "interrupt":
              // Human-in-the-loop interrupt
              toast.error(e.message ?? "Action requires approval");
              break;

            case "error":
              setMessages((m) =>
                appendToAssistant(
                  m,
                  `\n\n⚠️ ${e.message ?? "An error occurred."}`,
                ),
              );
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
      );
    } catch (err) {
      if ((err as Error)?.name !== "AbortError") {
        setMessages((m) =>
          appendToAssistant(
            m,
            "\n\n⚠️ Could not send message. Please try again.",
          ),
        );
        setInput((prev) => prev || content);
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
    <main className="flex min-w-0 flex-1 flex-col">
      {/* Streaming indicator bar */}
      {streaming && (
        <div className="flex items-center justify-between border-b border-line bg-gblue-soft/30 px-4 py-1.5 sm:px-6 dark:bg-gblue-soft/20">
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

      {/* Messages area — min-h-0 required for flex shrink + overflow */}
      <div
        ref={scrollContainerRef}
        onScroll={handleScroll}
        className="scroll-fine min-h-0 flex-1 overflow-y-auto"
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
        />
      </div>
    </main>
  );
}
