import { useEffect, useRef, useState } from "react";
import {
  createConversation,
  listConversations,
  getMessages,
  streamChat,
  type Conversation,
  type Message,
} from "../lib/api";
import Sidebar, { type View } from "./Sidebar";
import MessageBubble from "./MessageBubble";
import Composer from "./Composer";
import EmptyState from "./EmptyState";
import DocumentsView from "./DocumentsView";
import { MenuIcon, SparkIcon } from "./icons";

// Nhãn thân thiện cho từng tool khi agent đang gọi
const TOOL_LABELS: Record<string, string> = {
  ragSearch: "🔍 Đang tìm trong tài liệu…",
  createTask: "📝 Đang tạo task…",
  listTasks: "📋 Đang xem danh sách task…",
  updateTask: "✏️ Đang cập nhật task…",
  deleteTask: "🗑️ Đang xóa task…",
};

export default function ChatView() {
  const [view, setView] = useState<View>("chat");
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [streaming, setStreaming] = useState(false);
  const [toolStatus, setToolStatus] = useState<string | null>(null);
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

    await streamChat(convId, content, (e) => {
      if (e.type === "tool_start") {
        setToolStatus(TOOL_LABELS[e.name ?? ""] ?? `⚙️ ${e.name}…`);
      } else if (e.type === "tool_end") {
        setToolStatus(null);
      } else if (e.token) {
        setToolStatus(null);
        setMessages((m) => {
          const copy = [...m];
          const last = copy[copy.length - 1];
          copy[copy.length - 1] = {
            role: "assistant",
            content: last.content + e.token,
          };
          return copy;
        });
      }
    });
    setToolStatus(null);
    setStreaming(false);
  };

  const hasMessages = messages.length > 0;

  return (
    <div className="flex h-screen overflow-hidden">
      <Sidebar
        conversations={conversations}
        activeId={activeId}
        open={sidebarOpen}
        view={view}
        onSelect={(id) => {
          setActiveId(id);
          setView("chat");
          setSidebarOpen(false);
        }}
        onNew={startNew}
        onClose={() => setSidebarOpen(false)}
        onViewChange={(v) => {
          setView(v);
          setSidebarOpen(false);
        }}
      />

      {view === "documents" ? (
        <DocumentsView onOpenSidebar={() => setSidebarOpen(true)} />
      ) : (
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

          {/* Badge agent đang gọi tool */}
          {toolStatus && (
            <div className="px-4 sm:px-6">
              <div className="mx-auto flex max-w-3xl items-center gap-2 pb-1">
                <span className="inline-flex items-center gap-2 rounded-full border border-accent/30 bg-accent-soft px-3 py-1 text-xs font-medium text-accent-ink">
                  <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-accent" />
                  {toolStatus}
                </span>
              </div>
            </div>
          )}

          {/* Ô soạn tin */}
          <Composer
            value={input}
            onChange={setInput}
            onSend={() => send()}
            disabled={streaming}
          />
        </main>
      )}
    </div>
  );
}
