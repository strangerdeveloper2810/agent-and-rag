import { useState, useMemo } from "react";
import {
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

function ConversationSkeleton() {
  return (
    <div className="flex items-center gap-3 px-4 py-2.5">
      <div className="h-4 w-4 shrink-0 rounded-full skeleton" />
      <div className="h-4 flex-1 rounded-full skeleton" />
    </div>
  );
}

function JARVIS_Spark() {
  return (
    <svg
      width={22}
      height={22}
      viewBox="0 0 24 24"
      fill="none"
      stroke="var(--primary)"
      strokeWidth={1.5}
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M12 3.5c.6 3.7 1.8 4.9 5.5 5.5-3.7.6-4.9 1.8-5.5 5.5-.6-3.7-1.8-4.9-5.5-5.5 3.7-.6 4.9-1.8 5.5-5.5Z" />
      <path d="M18 14.5c.3 1.6.8 2.1 2.4 2.4-1.6.3-2.1.8-2.4 2.4-.3-1.6-.8-2.1-2.4-2.4 1.6-.3 2.1-.8 2.4-2.4Z" />
    </svg>
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

  const filtered = useMemo(() => {
    if (!searchQuery.trim()) return conversations;
    const q = searchQuery.toLowerCase();
    return conversations.filter((c) => c.title.toLowerCase().includes(q));
  }, [conversations, searchQuery]);

  const navItemClass = (active: boolean) => {
    const base =
      "flex w-full items-center gap-3 rounded-lg px-4 py-2.5 text-xs transition-all duration-200";
    if (active) {
      return `${base} font-medium border-l-2 shadow-[0_0_10px_rgba(0,240,255,0.15)]`;
    }
    return `${base} border-l-2 border-transparent hover:bg-[var(--border)] hover:border-[var(--primary-soft)]`;
  };

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
          className="fixed inset-0 z-20 bg-black/60 md:hidden"
          onClick={onClose}
          aria-hidden
        />
      )}

      <aside
        className={`fixed inset-y-0 left-0 z-30 flex w-[84vw] max-w-[300px] flex-col transition-all duration-300 md:static md:max-w-none ${
          open ? "translate-x-0" : "-translate-x-full"
        } ${
          collapsed
            ? "md:w-0 md:-translate-x-full md:overflow-hidden"
            : "md:w-[280px] md:translate-x-0"
        }`}
        style={{
          height: "100%",
          backgroundColor: "var(--surface)",
          borderRight: "1px solid var(--border)",
        }}
      >
        {/* Brand */}
        <div
          className="flex items-center justify-between px-5 py-4"
          style={{ borderBottom: "1px solid var(--border)" }}
        >
          <div className="flex items-center gap-2.5">
            <JARVIS_Spark />
            <p className="text-base font-medium tracking-wider animate-neon-pulse" style={{ color: "var(--primary)" }}>
              J.A.R.V.I.S.
            </p>
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label="Close menu"
            className="rounded-full p-1.5 transition hover:bg-[var(--border)] md:hidden"
            style={{ color: "var(--text-soft)" }}
          >
            <CloseIcon />
          </button>
        </div>

        {/* New conversation */}
        <div className="px-3 pt-3">
          <button
            type="button"
            onClick={onNew}
            className="flex w-full items-center gap-2 rounded-lg border px-4 py-2.5 text-xs font-medium transition-all duration-200 hover:shadow-[0_0_10px_rgba(0,240,255,0.2)]"
            style={{
              borderColor: "var(--primary)",
              color: "var(--primary)",
              backgroundColor: "transparent",
            }}
          >
            <PlusIcon width={16} height={16} />
            New chat
          </button>
        </div>

        {/* Nav */}
        <nav className="mt-3 space-y-0.5 px-3">
          <button
            type="button"
            onClick={() => onViewChange("chat")}
            className={navItemClass(view === "chat")}
            style={
              view === "chat"
                ? {
                    color: "var(--primary)",
                    borderLeftColor: "var(--primary)",
                  }
                : { color: "var(--text-soft)" }
            }
          >
            <ChatIcon width={16} height={16} />
            Chat
          </button>
          <button
            type="button"
            onClick={() => onViewChange("documents")}
            className={navItemClass(view === "documents")}
            style={
              view === "documents"
                ? {
                    color: "var(--primary)",
                    borderLeftColor: "var(--primary)",
                  }
                : { color: "var(--text-soft)" }
            }
          >
            <DocIcon width={16} height={16} />
            Documents
          </button>
        </nav>

        {/* Conversation list */}
        <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
          {view === "chat" && (
            <>
              <p
                className="px-6 pb-1 pt-5 text-[10px] font-medium uppercase tracking-widest"
                style={{ color: "var(--text-muted)" }}
              >
                Recent
              </p>

              {/* Search */}
              <div className="px-3 pb-2 shrink-0">
                <div
                  className="flex items-center gap-2 rounded-lg px-3 py-1.5 transition"
                  style={{
                    backgroundColor: "var(--border)",
                    border: "1px solid var(--border)",
                  }}
                >
                  <SearchIcon
                    width={14}
                    height={14}
                    style={{ color: "var(--text-muted)" }}
                    className="shrink-0"
                  />
                  <input
                    type="text"
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    placeholder="Search..."
                    aria-label="Search conversations"
                    className="flex-1 bg-transparent text-xs outline-none"
                    style={{ color: "var(--text)" }}
                  />
                  {searchQuery && (
                    <button
                      type="button"
                      onClick={() => setSearchQuery("")}
                      aria-label="Clear search"
                      className="rounded-full p-0.5 transition hover:text-[var(--text)]"
                      style={{ color: "var(--text-muted)" }}
                    >
                      <CloseIcon width={12} height={12} />
                    </button>
                  )}
                </div>
              </div>

              {/* List */}
              <nav className="scroll-fine min-h-0 flex-1 space-y-0.5 overflow-y-auto px-3 pb-4">
                {loading ? (
                  <div className="space-y-1">
                    {Array.from({ length: 5 }).map((_, i) => (
                      <ConversationSkeleton key={i} />
                    ))}
                  </div>
                ) : filtered.length === 0 ? (
                  <p
                    className="px-3 py-4 text-center text-[11px]"
                    style={{ color: "var(--text-muted)" }}
                  >
                    {searchQuery
                      ? "No conversations found."
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
                          className="flex items-center gap-2 rounded-lg px-3 py-1"
                          style={{
                            backgroundColor: "var(--primary-soft)",
                            borderLeft: "2px solid var(--primary)",
                          }}
                        >
                          <ChatIcon
                            width={14}
                            height={14}
                            className="shrink-0"
                            style={{ color: "var(--primary)" }}
                          />
                          <input
                            type="text"
                            value={renameValue}
                            onChange={(e) => setRenameValue(e.target.value)}
                            onKeyDown={handleRenameKeyDown}
                            onBlur={commitRename}
                            autoFocus
                            className="min-w-0 flex-1 bg-transparent text-xs outline-none"
                            style={{ color: "var(--text)" }}
                          />
                          <button
                            type="button"
                            onClick={commitRename}
                            aria-label="Confirm rename"
                            className="shrink-0 rounded-full p-1 transition"
                            style={{ color: "var(--primary)" }}
                          >
                            <CheckIcon width={13} height={13} />
                          </button>
                        </div>
                      );
                    }

                    return (
                      <div
                        key={c._id}
                        className={`group/item flex w-full items-center gap-2 rounded-lg pr-2 transition-all duration-150 ${
                          active
                            ? "shadow-[0_0_10px_rgba(0,240,255,0.1)]"
                            : "hover:bg-[var(--border)]"
                        }`}
                        style={
                          active
                            ? {
                                backgroundColor: "var(--primary-soft)",
                                borderLeft: "2px solid var(--primary)",
                              }
                            : { borderLeft: "2px solid transparent" }
                        }
                      >
                        <button
                          onClick={() => onSelect(c._id)}
                          className="flex min-w-0 flex-1 items-center gap-3 py-2 pl-4 text-left text-xs"
                          style={{ color: active ? "var(--primary)" : "var(--text-soft)" }}
                        >
                          <ChatIcon
                            width={14}
                            height={14}
                            className="shrink-0 opacity-70"
                          />
                          <span className="truncate">{c.title}</span>
                        </button>

                        <button
                          type="button"
                          onClick={() => startRename(c)}
                          aria-label={`Rename "${c.title}"`}
                          className="shrink-0 rounded-full p-1.5 opacity-0 transition hover:bg-[var(--border)] focus:opacity-100 group-hover/item:opacity-100"
                          style={{ color: "var(--text-muted)" }}
                        >
                          <EditIcon width={12} height={12} />
                        </button>

                        <button
                          type="button"
                          onClick={() => setPendingDelete(c)}
                          aria-label={`Delete "${c.title}"`}
                          className="shrink-0 rounded-full p-1.5 opacity-0 transition hover:bg-[rgba(255,51,102,0.15)] focus:opacity-100 group-hover/item:opacity-100"
                          style={{ color: "var(--text-muted)" }}
                        >
                          <TrashIcon width={14} height={14} />
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
