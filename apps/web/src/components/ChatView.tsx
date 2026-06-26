import { useEffect, useRef, useState } from "react";
import {
  createConversation,
  listConversations,
  getMessages,
  streamChat,
  type Conversation,
  type Message,
} from "../lib/api";

export default function ChatView() {
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [streaming, setStreaming] = useState(false);
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

  const send = async () => {
    if (!input.trim() || streaming) return;
    const content = input.trim();
    setInput("");

    let convId = activeId;
    if (!convId) {
      const conv = await createConversation(content);
      convId = conv._id;
      setActiveId(convId);
      setConversations((c) => [conv, ...c]);
    }

    setMessages((m) => [...m, { role: "user", content }]);
    setMessages((m) => [...m, { role: "assistant", content: "" }]);
    setStreaming(true);

    await streamChat(convId, content, (token) => {
      setMessages((m) => {
        const copy = [...m];
        copy[copy.length - 1] = {
          role: "assistant",
          content: copy[copy.length - 1].content + token,
        };
        return copy;
      });
    });
    setStreaming(false);
  };

  return (
    <div className="flex h-screen">
      {/* Sidebar hội thoại */}
      <aside className="w-64 border-r bg-gray-50 p-3 overflow-y-auto">
        <button
          className="w-full mb-3 rounded bg-blue-600 text-white py-2 text-sm"
          onClick={() => {
            setActiveId(null);
            setMessages([]);
          }}
        >
          + Hội thoại mới
        </button>
        {conversations.map((c) => (
          <button
            key={c._id}
            onClick={() => setActiveId(c._id)}
            className={`block w-full text-left text-sm p-2 rounded truncate ${
              c._id === activeId ? "bg-blue-100" : "hover:bg-gray-100"
            }`}
          >
            {c.title}
          </button>
        ))}
      </aside>

      {/* Khung chat */}
      <main className="flex-1 flex flex-col">
        <div className="flex-1 overflow-y-auto p-4 space-y-3">
          {messages.map((m, i) => (
            <div
              key={i}
              className={`max-w-2xl rounded-lg p-3 ${
                m.role === "user"
                  ? "ml-auto bg-blue-600 text-white"
                  : "bg-gray-100 text-gray-800"
              }`}
            >
              {m.content || (streaming ? "…" : "")}
            </div>
          ))}
          <div ref={endRef} />
        </div>
        <div className="border-t p-3 flex gap-2">
          <input
            className="flex-1 border rounded px-3 py-2"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && send()}
            placeholder="Nhập tin nhắn..."
            disabled={streaming}
          />
          <button
            onClick={send}
            disabled={streaming}
            className="rounded bg-blue-600 text-white px-4 disabled:opacity-50"
          >
            Gửi
          </button>
        </div>
      </main>
    </div>
  );
}
