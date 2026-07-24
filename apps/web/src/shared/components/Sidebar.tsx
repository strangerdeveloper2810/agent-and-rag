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
    <div className="flex items-center gap-3 px-4 py-2">
      <div className="h-3.5 w-3.5 shrink-0 rounded skeleton" />
      <div className="h-3.5 flex-1 rounded skeleton" />
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

  const filtered = useMemo(() => {
    if (!searchQuery.trim()) return conversations;
    const q = searchQuery.toLowerCase();
    return conversations.filter((c) => c.title.toLowerCase().includes(q));
  }, [conversations, searchQuery]);

  const navItemClass = (active: boolean) => {
    const base =
      "flex w-full items-center gap-2.5 rounded-md px-3 py-2 text-[12px] transition-colors duration-150";
    if (active) return `${base} font-medium`;
    return `${base} hover:bg-[var(--bg-raised)]`;
  };

  const startRename = (c: Conversation) => {
    setRenamingId(c._id);
    setRenameValue(c.title);
  };
  const commitRename = () => {
    if (renamingId && renameValue.trim())
      onRename(renamingId, renameValue.trim());
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
      {open && (
        <div
          className="fixed inset-0 z-20 bg-black/40 md:hidden"
          onClick={onClose}
          aria-hidden
        />
      )}

      <aside
        className={`fixed inset-y-0 left-0 z-30 flex w-[84vw] max-w-[260px] flex-col transition-all duration-200 md:static md:max-w-none ${
          open ? "translate-x-0" : "-translate-x-full"
        } ${collapsed ? "md:w-0 md:-translate-x-full md:overflow-hidden" : "md:w-[260px] md:translate-x-0"}`}
        style={{
          height: "100%",
          backgroundColor: "var(--surface)",
          borderRight: "1px solid var(--border)",
        }}
      >
        {/* Brand */}
        <div
          className="flex items-center justify-between px-4 py-3.5"
          style={{ borderBottom: "1px solid var(--border)" }}
        >
          <div className="flex items-center gap-2">
            <div
              className="flex h-7 w-7 items-center justify-center rounded-md"
              style={{ backgroundColor: "var(--accent)" }}
            >
              <svg
                width={14}
                height={14}
                viewBox="0 0 24 24"
                fill="none"
                stroke="#fff"
                strokeWidth={2}
                strokeLinecap="round"
              >
                <path d="M12 3.5c.6 3.7 1.8 4.9 5.5 5.5-3.7.6-4.9 1.8-5.5 5.5-.6-3.7-1.8-4.9-5.5-5.5 3.7-.6 4.9-1.8 5.5-5.5Z" />
              </svg>
            </div>
            <p
              className="text-[13px] font-semibold tracking-tight"
              style={{ color: "var(--text)" }}
            >
              J.A.R.V.I.S.
            </p>
          </div>
          <button
            onClick={onClose}
            aria-label="Close menu"
            className="rounded p-1 transition hover:bg-[var(--bg-raised)] md:hidden"
            style={{ color: "var(--text-secondary)" }}
          >
            <CloseIcon />
          </button>
        </div>

        {/* New chat */}
        <div className="px-3 pt-3">
          <button
            onClick={onNew}
            className="flex w-full items-center gap-2 rounded-md border px-3 py-2 text-[12px] font-medium transition-colors duration-150 hover:bg-[var(--bg-raised)]"
            style={{ borderColor: "var(--border)", color: "var(--text)" }}
          >
            <PlusIcon width={14} height={14} />
            New chat
          </button>
        </div>

        {/* Nav */}
        <nav className="mt-2 space-y-0.5 px-3">
          <button
            onClick={() => onViewChange("chat")}
            className={navItemClass(view === "chat")}
            style={
              view === "chat"
                ? {
                    color: "var(--accent)",
                    backgroundColor: "var(--accent-bg)",
                  }
                : { color: "var(--text-secondary)" }
            }
          >
            <ChatIcon width={14} height={14} /> Chat
          </button>
          <button
            onClick={() => onViewChange("documents")}
            className={navItemClass(view === "documents")}
            style={
              view === "documents"
                ? {
                    color: "var(--accent)",
                    backgroundColor: "var(--accent-bg)",
                  }
                : { color: "var(--text-secondary)" }
            }
          >
            <DocIcon width={14} height={14} /> Documents
          </button>
        </nav>

        {/* Conversations */}
        <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
          {view === "chat" && (
            <>
              <p
                className="px-5 pb-1.5 pt-4 text-[10px] font-semibold uppercase tracking-widest"
                style={{ color: "var(--text-tertiary)" }}
              >
                Recent
              </p>

              <div className="px-3 pb-2 shrink-0">
                <div
                  className="flex items-center gap-2 rounded-md border px-2.5 py-1.5"
                  style={{
                    borderColor: "var(--border)",
                    backgroundColor: "var(--bg-raised)",
                  }}
                >
                  <SearchIcon
                    width={13}
                    height={13}
                    style={{ color: "var(--text-tertiary)" }}
                    className="shrink-0"
                  />
                  <input
                    type="text"
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    placeholder="Search..."
                    aria-label="Search conversations"
                    className="flex-1 bg-transparent text-[11px] outline-none placeholder:text-[var(--text-tertiary)]"
                    style={{ color: "var(--text)" }}
                  />
                  {searchQuery && (
                    <button
                      onClick={() => setSearchQuery("")}
                      aria-label="Clear"
                      className="rounded p-0.5"
                      style={{ color: "var(--text-tertiary)" }}
                    >
                      <CloseIcon width={11} height={11} />
                    </button>
                  )}
                </div>
              </div>

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
                    style={{ color: "var(--text-tertiary)" }}
                  >
                    {searchQuery ? "No results." : "No conversations yet."}
                  </p>
                ) : (
                  filtered.map((c) => {
                    const active = c._id === activeId;
                    if (renamingId === c._id) {
                      return (
                        <div
                          key={c._id}
                          className="flex items-center gap-2 rounded-md px-3 py-1"
                          style={{ backgroundColor: "var(--accent-bg)" }}
                        >
                          <ChatIcon
                            width={14}
                            height={14}
                            style={{ color: "var(--accent)" }}
                            className="shrink-0"
                          />
                          <input
                            type="text"
                            value={renameValue}
                            onChange={(e) => setRenameValue(e.target.value)}
                            onKeyDown={handleRenameKeyDown}
                            onBlur={commitRename}
                            autoFocus
                            className="min-w-0 flex-1 bg-transparent text-[12px] outline-none"
                            style={{ color: "var(--text)" }}
                          />
                          <button
                            onClick={commitRename}
                            aria-label="Confirm"
                            className="shrink-0 rounded p-0.5"
                            style={{ color: "var(--accent)" }}
                          >
                            <CheckIcon width={13} height={13} />
                          </button>
                        </div>
                      );
                    }
                    return (
                      <div
                        key={c._id}
                        className={`group/item flex w-full items-center gap-2 rounded-md pr-1.5 transition-colors duration-150 ${
                          active
                            ? "bg-[var(--accent-bg)]"
                            : "hover:bg-[var(--bg-raised)]"
                        }`}
                      >
                        <button
                          onClick={() => onSelect(c._id)}
                          className="flex min-w-0 flex-1 items-center gap-2.5 py-2 pl-3 text-left"
                        >
                          <ChatIcon
                            width={13}
                            height={13}
                            className="shrink-0"
                            style={{
                              color: active
                                ? "var(--accent)"
                                : "var(--text-tertiary)",
                            }}
                          />
                          <span
                            className="truncate text-[12px]"
                            style={{
                              color: active ? "var(--accent)" : "var(--text)",
                            }}
                          >
                            {c.title}
                          </span>
                        </button>
                        <button
                          onClick={() => startRename(c)}
                          aria-label={`Rename`}
                          className="shrink-0 rounded p-1 opacity-0 transition hover:bg-[var(--border)] focus:opacity-100 group-hover/item:opacity-100"
                          style={{ color: "var(--text-tertiary)" }}
                        >
                          <EditIcon width={12} height={12} />
                        </button>
                        <button
                          onClick={() => setPendingDelete(c)}
                          aria-label={`Delete`}
                          className="shrink-0 rounded p-1 opacity-0 transition hover:bg-red-50 hover:text-[var(--danger)] focus:opacity-100 group-hover/item:opacity-100"
                          style={{ color: "var(--text-tertiary)" }}
                        >
                          <TrashIcon width={13} height={13} />
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
            ? `"${pendingDelete.title}" will be permanently deleted.`
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
