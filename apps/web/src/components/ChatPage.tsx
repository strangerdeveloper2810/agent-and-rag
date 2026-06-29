import { useEffect, useRef, useState } from "react";
import { useNavigate, useOutletContext, useParams } from "react-router-dom";
import {
  createConversation,
  getMessages,
  streamChat,
  type Message,
} from "../lib/api";
import type { OutletCtx } from "./AppLayout";
import MessageBubble from "./MessageBubble";
import Composer from "./Composer";
import EmptyState from "./EmptyState";
import { MenuIcon } from "./icons";

const TOOL_LABELS: Record<string, string> = {
  ragSearch: "🔍 Đang tìm trong tài liệu…",
  listDocuments: "📚 Đang xem danh sách tài liệu…",
  readDocument: "📖 Đang đọc tài liệu…",
  createTask: "📝 Đang tạo task…",
  listTasks: "📋 Đang xem danh sách task…",
  updateTask: "✏️ Đang cập nhật task…",
  deleteTask: "🗑️ Đang xóa task…",
};

export default function ChatPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const { reloadConversations, openSidebar } = useOutletContext<OutletCtx>();

  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [streaming, setStreaming] = useState(false);
  const [toolStatus, setToolStatus] = useState<string | null>(null);
  const endRef = useRef<HTMLDivElement>(null);
  const loadedIdRef = useRef<string | null>(null);

  // Load tin nhắn theo id trên URL. Bỏ qua conv vừa tự tạo (đang stream dở).
  useEffect(() => {
    if (!id) {
      setMessages([]);
      loadedIdRef.current = null;
      return;
    }
    if (id === loadedIdRef.current) return;
    loadedIdRef.current = id;
    getMessages(id).then(setMessages);
  }, [id]);

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  const send = async (text?: string) => {
    const content = (text ?? input).trim();
    if (!content || streaming) return;
    setInput("");

    let convId = id;
    if (!convId) {
      const conv = await createConversation(content);
      convId = conv._id;
      loadedIdRef.current = convId; // tránh effect fetch lại khi URL đổi
      navigate(`/messages/${convId}`);
      // Chỉ reload sidebar khi tạo hội thoại MỚI (để nó hiện ngay trong "Gần đây")
      reloadConversations();
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
    <main className="flex min-w-0 flex-1 flex-col">
      {/* Header tối giản */}
      <header className="flex items-center gap-3 px-4 py-3 sm:px-6">
        <button
          type="button"
          onClick={openSidebar}
          aria-label="Mở menu"
          className="rounded-full p-2 text-ink-soft hover:bg-subtle md:hidden"
        >
          <MenuIcon />
        </button>
        <h1 className="font-medium text-ink">
          Agent <span className="text-gemini font-semibold">Tut</span>
        </h1>
        {streaming && (
          <span className="ml-auto flex items-center gap-1.5 text-xs text-gblue">
            <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-gblue" />
            đang trả lời…
          </span>
        )}
      </header>

      {/* Vùng tin nhắn */}
      <div className="scroll-fine flex-1 overflow-y-auto">
        {hasMessages ? (
          <div className="mx-auto max-w-3xl space-y-7 px-4 py-6 sm:px-6">
            {messages.map((m, i) => (
              <MessageBubble
                key={m._id ?? i}
                message={m}
                streaming={
                  streaming && i === messages.length - 1 && m.role === "assistant"
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
          <div className="mx-auto flex max-w-3xl pb-1">
            <span className="inline-flex items-center gap-2 rounded-full bg-gblue-soft px-3 py-1 text-xs font-medium text-gblue">
              <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-gblue" />
              {toolStatus}
            </span>
          </div>
        </div>
      )}

      <Composer
        value={input}
        onChange={setInput}
        onSend={() => send()}
        disabled={streaming}
      />
    </main>
  );
}
