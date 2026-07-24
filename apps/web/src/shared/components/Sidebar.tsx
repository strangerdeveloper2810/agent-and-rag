import { useState, useMemo } from "react";
import {
  SparkIcon,
  PlusIcon,
  ChatIcon,
  DocIcon,
  CloseIcon,
  TrashIcon,
  SearchIcon,
  EditIcon,
  CheckIcon,
} from "./icons";
import ConfirmDialog from "./ConfirmDialog";
import type { Conversation } from "@/modules/chat/chat.api";

export type View = "chat" | "documents";

interface SidebarProps {
  conversations: Conversation[];
  loading: boolean;
  activeId: string | null;
  open: boolean;
  collapsed: boolean;
  view: View;
  onSelect: (id: string) => void;
  onNew: () => void;
  onClose: () => void;
  onViewChange: (v: View) => void;
  onDelete: (id: string) => void;
  onRename: (id: string, title: string) => void;
}

/** Loading skeleton for conversation list items. */
function ConversationSkeleton() {
  return (
    <div className="flex items-center gap-3 px-4 py-2.5">
      <div className="h-4 w-4 shrink-0 rounded-full skeleton" />
      <div className="h-4 flex-1 rounded-full skeleton" />
    </div>
  );
}

export default function Sidebar({
  conversations,
  loading,
  activeId,
  open,
  collapsed,
  view,
  onSelect,
  onNew,
  onClose,
  onViewChange,
  onDelete,
  onRename,
}: SidebarProps) {
  const [pendingDelete, setPendingDelete] = useState<Conversation | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [renamingId, setRenamingId] = useState<string | null>(null);
  const [renameValue, setRenameValue] = useState("");

  // Filter conversations by search
  const filtered = useMemo(() => {
    if (!searchQuery.trim()) return conversations;
    const q = searchQuery.toLowerCase();
    return conversations.filter((c) => c.title.toLowerCase().includes(q));
  }, [conversations, searchQuery]);

  const navItemClass = (active: boolean) =>
    `flex w-full items-center gap-3 rounded-full px-4 py-2.5 text-sm transition ${
      active
        ? "bg-gblue-soft font-medium text-gblue"
        : "text-ink-soft hover:bg-subtle"
    }`;

  const startRename = (c: Conversation) => {
    setRenamingId(c._id);
    setRenameValue(c.title);
  };

  const commitRename = () => {
    if (renamingId && renameValue.trim()) {
      onRename(renamingId, renameValue.trim());
    }
    setRenamingId(null);
    setRenameValue("");
  };

  const handleRenameKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") commitRename();
    if (e.key === "Escape") {
      setRenamingId(null);
      setRenameValue("");
    }
  };

  return (
    <>
      {/* Mobile overlay */}
      {open && (
        <div
          className="fixed inset-0 z-20 bg-black/30 md:hidden"
          onClick={onClose}
          aria-hidden
        />
      )}

      <aside
        className={`fixed inset-y-0 left-0 z-30 flex h-full w-[84vw] max-w-[320px] flex-col bg-subtle transition-all duration-300 md:static md:max-w-none ${
          open ? "translate-x-0" : "-translate-x-full"
        } ${
          collapsed
            ? "md:w-0 md:-translate-x-full md:overflow-hidden"
            : "md:w-[300px] md:translate-x-0"
        }`}
      >
        {/* Brand */}
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
            aria-label="Close menu"
            className="rounded-full p-1.5 text-ink-soft hover:bg-subtle2 md:hidden"
          >
            <CloseIcon />
          </button>
        </div>

        {/* New conversation */}
        <div className="px-3">
          <button
            type="button"
            onClick={onNew}
            className="flex items-center gap-2 rounded-full bg-subtle2 px-4 py-2.5 text-sm font-medium text-ink-soft transition hover:bg-line hover:shadow-soft"
          >
            <PlusIcon width={18} height={18} />
            New conversation
          </button>
        </div>

        {/* Nav: Chat / Documents */}
        <nav className="mt-4 space-y-1 px-3">
          <button
            type="button"
            onClick={() => onViewChange("chat")}
            className={navItemClass(view === "chat")}
          >
            <ChatIcon width={18} height={18} />
            Chat
          </button>
          <button
            type="button"
            onClick={() => onViewChange("documents")}
            className={navItemClass(view === "documents")}
          >
            <DocIcon width={18} height={18} />
            Documents
          </button>
        </nav>

        {/* Conversation list (only in Chat view) — fills remaining sidebar height */}
        <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
          {view === "chat" && (
            <>
              <p className="px-6 pb-1 pt-5 text-xs font-medium text-ink-faint">
                Recent
              </p>

              {/* Search */}
              <div className="px-3 pb-2 shrink-0">
                <div className="flex items-center gap-2 rounded-full bg-subtle2 px-3 py-1.5 ring-1 ring-line/50 transition focus-within:ring-gblue/30">
                  <SearchIcon
                    width={15}
                    height={15}
                    className="shrink-0 text-ink-faint"
                  />
                  <input
                    type="text"
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    placeholder="Search conversations..."
                    aria-label="Search conversations"
                    className="flex-1 bg-transparent text-sm text-ink outline-none placeholder:text-ink-faint"
                  />
                  {searchQuery && (
                    <button
                      type="button"
                      onClick={() => setSearchQuery("")}
                      aria-label="Clear search"
                      className="rounded-full p-0.5 text-ink-faint hover:text-ink"
                    >
                      <CloseIcon width={13} height={13} />
                    </button>
                  )}
                </div>
              </div>

              {/* List — takes remaining space */}
              <nav className="scroll-fine min-h-0 flex-1 space-y-0.5 overflow-y-auto px-3 pb-4">
                {loading ? (
                  <div className="space-y-1">
                    {Array.from({ length: 5 }).map((_, i) => (
                      <ConversationSkeleton key={i} />
                    ))}
                  </div>
                ) : filtered.length === 0 ? (
                  <p className="px-3 py-4 text-center text-xs text-ink-faint">
                    {searchQuery
                      ? "No conversations match your search."
                      : "No conversations yet."}
                  </p>
                ) : (
                  filtered.map((c) => {
                    const active = c._id === activeId;
                    const isRenaming = renamingId === c._id;

                    if (isRenaming) {
                      return (
                        <div
                          key={c._id}
                          className="flex items-center gap-2 rounded-full bg-gblue-soft px-3 py-1"
                        >
                          <ChatIcon
                            width={16}
                            height={16}
                            className="shrink-0 text-gblue"
                          />
                          <input
                            type="text"
                            value={renameValue}
                            onChange={(e) => setRenameValue(e.target.value)}
                            onKeyDown={handleRenameKeyDown}
                            onBlur={commitRename}
                            autoFocus
                            className="min-w-0 flex-1 bg-transparent text-sm text-ink outline-none"
                          />
                          <button
                            type="button"
                            onClick={commitRename}
                            aria-label="Confirm rename"
                            className="shrink-0 rounded-full p-1 text-gblue hover:bg-gblue-soft/50"
                          >
                            <CheckIcon width={14} height={14} />
                          </button>
                        </div>
                      );
                    }

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

                        {/* Rename button */}
                        <button
                          type="button"
                          onClick={() => startRename(c)}
                          aria-label={`Rename "${c.title}"`}
                          className="shrink-0 rounded-full p-1.5 text-ink-faint opacity-0 transition hover:bg-subtle2 focus:opacity-100 group-hover/item:opacity-100"
                        >
                          <EditIcon width={13} height={13} />
                        </button>

                        {/* Delete button */}
                        <button
                          type="button"
                          onClick={() => setPendingDelete(c)}
                          aria-label={`Delete "${c.title}"`}
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
        </div>
      </aside>

      <ConfirmDialog
        open={pendingDelete !== null}
        title="Delete conversation?"
        message={
          pendingDelete
            ? `"${pendingDelete.title}" and all its messages will be permanently deleted.`
            : undefined
        }
        confirmLabel="Delete"
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
