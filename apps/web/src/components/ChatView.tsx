import { useEffect, useRef, useState } from "react";
import {
  createConversation,
  listConversations,
  getMessages,
  streamChat,
  type Conversation,
  type Message,
} from "../lib/api";
import Sidebar from "./Sidebar";
import MessageBubble from "./MessageBubble";
import Composer from "./Composer";
import EmptyState from "./EmptyState";
import { MenuIcon, SparkIcon } from "./icons";

export default function ChatView() {
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [streaming, setStreaming] = useState(false);
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const endRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    listConversations().then(setConversations);
  }, []);

  useEffect(() => {
    if (activeId) getMessages(activeId).then(setMessages);
  }, [activeId]);

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  const startNew = () => {
    setActiveId(null);
    setMessages([]);
    setSidebarOpen(false);
  };

  // text tùy chọn: dùng khi bấm chip gợi ý (không phụ thuộc state input async)
  const send = async (text?: string) => {
    const content = (text ?? input).trim();
    if (!content || streaming) return;
    setInput("");

    let convId = activeId;
    if (!convId) {
      const conv = await createConversation(content);
      convId = conv._id;
      setActiveId(convId);
      setConversations((c) => [conv, ...c]);
    }

    setMessages((m) => [
      ...m,
      { role: "user", content },
      { role: "assistant", content: "" },
    ]);
    setStreaming(true);

    await streamChat(convId, content, (token) => {
      setMessages((m) => {
        const copy = [...m];
        const last = copy[copy.length - 1];
        copy[copy.length - 1] = { role: "assistant", content: last.content + token };
        return copy;
      });
    });
    setStreaming(false);
  };

  const hasMessages = messages.length > 0;

  return (
    <div className="flex h-screen overflow-hidden">
      <Sidebar
        conversations={conversations}
        activeId={activeId}
        open={sidebarOpen}
        onSelect={(id) => {
          setActiveId(id);
          setSidebarOpen(false);
        }}
        onNew={startNew}
        onClose={() => setSidebarOpen(false)}
      />

      <main className="flex min-w-0 flex-1 flex-col">
        {/* Thanh trên cùng (chủ yếu cho mobile + ngữ cảnh) */}
        <header className="flex items-center gap-3 border-b border-line/70 px-4 py-3 sm:px-6">
          <button
            type="button"
            onClick={() => setSidebarOpen(true)}
            aria-label="Mở menu"
            className="rounded-lg p-1.5 text-ink-soft hover:bg-line/60 md:hidden"
          >
            <MenuIcon />
          </button>
          <div className="flex items-center gap-2">
            <div className="flex h-6 w-6 items-center justify-center rounded-md bg-gradient-to-br from-accent-glow to-accent-ink text-white md:hidden">
              <SparkIcon width={13} height={13} />
            </div>
            <h2 className="truncate font-display text-sm font-semibold text-ink">
              {conversations.find((c) => c._id === activeId)?.title ??
                "Hội thoại mới"}
            </h2>
          </div>
          {streaming && (
            <span className="ml-auto flex items-center gap-1.5 text-xs text-accent-ink">
              <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-accent" />
              đang trả lời…
            </span>
          )}
        </header>

        {/* Vùng tin nhắn */}
        <div className="scroll-fine flex-1 overflow-y-auto">
          {hasMessages ? (
            <div className="mx-auto max-w-3xl space-y-6 px-4 py-6 sm:px-6">
              {messages.map((m, i) => (
                <MessageBubble
                  key={m._id ?? i}
                  message={m}
                  streaming={
                    streaming &&
                    i === messages.length - 1 &&
                    m.role === "assistant"
                  }
                />
              ))}
              <div ref={endRef} />
            </div>
          ) : (
            <EmptyState onPick={(p) => send(p)} />
          )}
        </div>

        {/* Ô soạn tin */}
        <Composer
          value={input}
          onChange={setInput}
          onSend={() => send()}
          disabled={streaming}
        />
      </main>
    </div>
  );
}
