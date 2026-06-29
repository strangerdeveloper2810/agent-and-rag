import { useState } from "react";
import {
  SparkIcon,
  PlusIcon,
  ChatIcon,
  DocIcon,
  CloseIcon,
  TrashIcon,
} from "./icons";
import ConfirmDialog from "./ConfirmDialog";
import type { Conversation } from "@/modules/chat/chat.api";

export type View = "chat" | "documents";

export default function Sidebar({
  conversations,
  activeId,
  open,
  collapsed,
  view,
  onSelect,
  onNew,
  onClose,
  onViewChange,
  onDelete,
}: {
  conversations: Conversation[];
  activeId: string | null;
  open: boolean;
  collapsed: boolean;
  view: View;
  onSelect: (id: string) => void;
  onNew: () => void;
  onClose: () => void;
  onViewChange: (v: View) => void;
  onDelete: (id: string) => void;
}) {
  // Hội thoại đang chờ xác nhận xóa (mở modal)
  const [pendingDelete, setPendingDelete] = useState<Conversation | null>(null);

  const navItem = (active: boolean) =>
    `flex w-full items-center gap-3 rounded-full px-4 py-2.5 text-sm transition ${
      active
        ? "bg-gblue-soft font-medium text-gblue"
        : "text-ink-soft hover:bg-subtle"
    }`;

  return (
    <>
      {open && (
        <div
          className="fixed inset-0 z-20 bg-black/30 md:hidden"
          onClick={onClose}
          aria-hidden
        />
      )}

      <aside
        className={`fixed inset-y-0 left-0 z-30 flex w-[84vw] max-w-[320px] flex-col overflow-hidden bg-subtle transition-all duration-300 md:static md:max-w-none ${
          open ? "translate-x-0" : "-translate-x-full"
        } ${
          collapsed
            ? "md:w-0 md:-translate-x-full"
            : "md:w-[300px] md:translate-x-0"
        }`}
      >
        {/* Thương hiệu */}
        <div className="flex items-center justify-between px-5 py-4">
          <div className="flex items-center gap-2.5">
            <div className="flex h-9 w-9 items-center justify-center rounded-full bg-gemini text-white">
              <SparkIcon width={18} height={18} />
            </div>
            <p className="text-lg font-medium text-ink">
              Agent <span className="text-gemini font-semibold">Tut</span>
            </p>
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label="Đóng menu"
            className="rounded-full p-1.5 text-ink-soft hover:bg-subtle2 md:hidden"
          >
            <CloseIcon />
          </button>
        </div>

        {/* Cuộc trò chuyện mới */}
        <div className="px-3">
          <button
            type="button"
            onClick={onNew}
            className="flex items-center gap-2 rounded-full bg-subtle2 px-4 py-2.5 text-sm font-medium text-ink-soft transition hover:bg-line hover:shadow-soft"
          >
            <PlusIcon width={18} height={18} />
            Cuộc trò chuyện mới
          </button>
        </div>

        {/* Nav: Trò chuyện / Tài liệu */}
        <nav className="mt-4 space-y-1 px-3">
          <button
            type="button"
            onClick={() => onViewChange("chat")}
            className={navItem(view === "chat")}
          >
            <ChatIcon width={18} height={18} />
            Trò chuyện
          </button>
          <button
            type="button"
            onClick={() => onViewChange("documents")}
            className={navItem(view === "documents")}
          >
            <DocIcon width={18} height={18} />
            Tài liệu
          </button>
        </nav>

        {/* Danh sách hội thoại gần đây — chỉ ở tab Trò chuyện */}
        {view === "chat" && (
          <>
            <p className="px-6 pb-1 pt-5 text-xs font-medium text-ink-faint">
              Gần đây
            </p>
            <nav className="scroll-fine flex-1 space-y-0.5 overflow-y-auto px-3 pb-4">
              {conversations.length === 0 ? (
                <p className="px-3 py-4 text-center text-xs text-ink-faint">
                  Chưa có hội thoại nào.
                </p>
              ) : (
                conversations.map((c) => {
                  const active = c._id === activeId;
                  return (
                    <div
                      key={c._id}
                      className={`group/item flex w-full items-center gap-2 rounded-full pr-2 transition ${
                        active
                          ? "bg-gblue-soft text-gblue"
                          : "text-ink-soft hover:bg-subtle2"
                      }`}
                    >
                      <button
                        onClick={() => onSelect(c._id)}
                        className="flex min-w-0 flex-1 items-center gap-3 py-2 pl-4 text-left text-sm"
                      >
                        <ChatIcon
                          width={16}
                          height={16}
                          className="shrink-0 opacity-70"
                        />
                        <span className="truncate">{c.title}</span>
                      </button>
                      <button
                        type="button"
                        onClick={() => setPendingDelete(c)}
                        aria-label={`Xóa hội thoại ${c.title}`}
                        className="shrink-0 rounded-full p-1.5 text-ink-faint opacity-0 transition hover:bg-red-100 hover:text-red-600 focus:opacity-100 group-hover/item:opacity-100"
                      >
                        <TrashIcon width={15} height={15} />
                      </button>
                    </div>
                  );
                })
              )}
            </nav>
          </>
        )}
      </aside>

      <ConfirmDialog
        open={pendingDelete !== null}
        title="Xóa hội thoại?"
        message={
          pendingDelete
            ? `Hội thoại "${pendingDelete.title}" và toàn bộ tin nhắn sẽ bị xóa vĩnh viễn.`
            : undefined
        }
        confirmLabel="Xóa"
        danger
        onConfirm={() => {
          if (pendingDelete) onDelete(pendingDelete._id);
          setPendingDelete(null);
        }}
        onCancel={() => setPendingDelete(null)}
      />
    </>
  );
}
