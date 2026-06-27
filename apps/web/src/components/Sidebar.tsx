import { SparkIcon, PlusIcon, ChatIcon, DocIcon, CloseIcon } from "./icons";
import type { Conversation } from "../lib/api";

export type View = "chat" | "documents";

export default function Sidebar({
  conversations,
  activeId,
  open,
  view,
  onSelect,
  onNew,
  onClose,
  onViewChange,
}: {
  conversations: Conversation[];
  activeId: string | null;
  open: boolean;
  view: View;
  onSelect: (id: string) => void;
  onNew: () => void;
  onClose: () => void;
  onViewChange: (v: View) => void;
}) {
  return (
    <>
      {/* Lớp phủ mờ khi mở drawer trên mobile */}
      {open && (
        <div
          className="fixed inset-0 z-20 bg-ink/30 backdrop-blur-sm md:hidden"
          onClick={onClose}
          aria-hidden
        />
      )}

      <aside
        className={`fixed inset-y-0 left-0 z-30 flex w-72 flex-col border-r border-line bg-surface/80 backdrop-blur-xl transition-transform duration-300 md:static md:translate-x-0 ${
          open ? "translate-x-0" : "-translate-x-full"
        }`}
      >
        {/* Thương hiệu */}
        <div className="flex items-center justify-between px-4 py-4">
          <div className="flex items-center gap-2.5">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-accent-glow to-accent-ink text-white shadow-bubble">
              <SparkIcon width={18} height={18} />
            </div>
            <div className="leading-tight">
              <p className="font-display text-base font-bold tracking-tight text-ink">
                Agent Tut
              </p>
              <p className="text-[11px] text-ink-faint">Trợ lý AI</p>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label="Đóng menu"
            className="rounded-lg p-1.5 text-ink-soft hover:bg-line/60 md:hidden"
          >
            <CloseIcon />
          </button>
        </div>

        {/* Nav: chuyển giữa Trò chuyện và Tài liệu */}
        <div className="flex gap-1 rounded-xl bg-line/40 p-1 mx-3">
          {([
            { key: "chat", label: "Trò chuyện", Icon: ChatIcon },
            { key: "documents", label: "Tài liệu", Icon: DocIcon },
          ] as const).map(({ key, label, Icon }) => (
            <button
              key={key}
              type="button"
              onClick={() => onViewChange(key)}
              className={`flex flex-1 items-center justify-center gap-1.5 rounded-lg px-2 py-1.5 text-sm font-medium transition ${
                view === key
                  ? "bg-surface text-ink shadow-ring"
                  : "text-ink-soft hover:text-ink"
              }`}
            >
              <Icon width={15} height={15} />
              {label}
            </button>
          ))}
        </div>

        {/* Hội thoại mới — chỉ ở tab Trò chuyện */}
        {view === "chat" && (
          <div className="mt-3 px-3">
            <button
              type="button"
              onClick={onNew}
              className="flex w-full items-center justify-center gap-2 rounded-2xl bg-ink px-4 py-2.5 text-sm font-medium text-paper shadow-soft transition hover:bg-accent-ink"
            >
              <PlusIcon width={16} height={16} />
              Hội thoại mới
            </button>
          </div>
        )}

        {/* Danh sách hội thoại — chỉ ở tab Trò chuyện */}
        <nav className="scroll-fine mt-4 flex-1 space-y-0.5 overflow-y-auto px-3 pb-4">
          {view === "chat" &&
            (conversations.length === 0 ? (
              <p className="px-3 py-6 text-center text-xs text-ink-faint">
                Chưa có hội thoại nào.
              </p>
            ) : (
              conversations.map((c) => {
                const active = c._id === activeId;
                return (
                  <button
                    key={c._id}
                    onClick={() => onSelect(c._id)}
                    className={`flex w-full items-center gap-2.5 rounded-xl px-3 py-2.5 text-left text-sm transition ${
                      active
                        ? "bg-accent-soft text-accent-ink"
                        : "text-ink-soft hover:bg-line/50 hover:text-ink"
                    }`}
                  >
                    <ChatIcon
                      width={16}
                      height={16}
                      className={active ? "text-accent" : "text-ink-faint"}
                    />
                    <span className="truncate">{c.title}</span>
                  </button>
                );
              })
            ))}
        </nav>
      </aside>
    </>
  );
}
