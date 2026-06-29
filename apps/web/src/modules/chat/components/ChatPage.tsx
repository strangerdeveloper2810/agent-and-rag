import { useEffect, useRef, useState } from "react";
import { useNavigate, useOutletContext, useParams } from "react-router-dom";
import {
  createConversation,
  getMessages,
  streamChat,
  type Message,
} from "@/modules/chat/chat.api";
import type { OutletCtx } from "@/shared/components/AppLayout";
import MessageBubble from "./MessageBubble";
import Composer from "./Composer";
import EmptyState from "./EmptyState";
import { MenuIcon } from "@/shared/components/icons";

export default function ChatPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const { reloadConversations, toggleSidebar } = useOutletContext<OutletCtx>();

  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [streaming, setStreaming] = useState(false);
  const [activeTool, setActiveTool] = useState<string | null>(null);
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
        setActiveTool(e.name ?? null);
      } else if (e.type === "tool_end") {
        setActiveTool(null);
      } else if (e.token) {
        setActiveTool(null);
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
    setActiveTool(null);
    setStreaming(false);
  };

  const hasMessages = messages.length > 0;

  return (
    <main className="flex min-w-0 flex-1 flex-col">
      {/* Header tối giản */}
      <header className="flex items-center gap-3 px-4 py-3 sm:px-6">
        <button
          type="button"
          onClick={toggleSidebar}
          aria-label="Ẩn/hiện menu"
          className="rounded-full p-2 text-ink-soft hover:bg-subtle"
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
                activeTool={
                  i === messages.length - 1 && m.role === "assistant"
                    ? activeTool
                    : null
                }
              />
            ))}
            <div ref={endRef} />
          </div>
        ) : (
          <EmptyState onPick={(p) => send(p)} />
        )}
      </div>

      <Composer
        value={input}
        onChange={setInput}
        onSend={() => send()}
        disabled={streaming}
      />
    </main>
  );
}
