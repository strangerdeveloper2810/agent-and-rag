import { useState, useMemo } from "react";
import {
  PlusIcon,
  ChatIcon,
  DocIcon,
  CloseIcon,
  TrashIcon,
  EditIcon,
  CheckIcon,
} from "@/shared/components/icons";
import ConfirmDialog from "@/shared/components/ConfirmDialog";
import { Button } from "../atoms/Button";
import { Kbd } from "../atoms/Kbd";
import { NavTab } from "../molecules/NavTab";
import { SearchBar } from "../molecules/SearchBar";
import type { Conversation } from "@/modules/chat/chat.api";

export type View = "chat" | "documents";

export interface SidebarProps {
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

/**
 * Skeleton loader component for conversation list items.
 */
const ConversationSkeleton: React.FC = () => {
  return (
    <div className="flex items-center gap-3 px-4 py-2">
      <div className="h-3.5 w-3.5 shrink-0 rounded skeleton" />
      <div className="h-3.5 flex-1 rounded skeleton" />
    </div>
  );
};

/**
 * Sidebar organism component rendering navigation drawer, conversation history,
 * search filter, and document management tab triggers.
 */
export const Sidebar: React.FC<SidebarProps> = ({
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
}) => {
  const [pendingDelete, setPendingDelete] = useState<Conversation | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [renamingId, setRenamingId] = useState<string | null>(null);
  const [renameValue, setRenameValue] = useState("");

  const filtered = useMemo(() => {
    if (!searchQuery.trim()) return conversations;
    const q = searchQuery.toLowerCase();
    return conversations.filter((c) => c.title.toLowerCase().includes(q));
  }, [conversations, searchQuery]);

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
          className="fixed inset-0 z-20 bg-black/60 backdrop-blur-sm md:hidden"
          onClick={onClose}
          aria-hidden
        />
      )}

      <aside
        className={`fixed inset-y-0 left-0 z-30 flex w-[85vw] max-w-[280px] flex-col transition-all duration-300 ease-out md:static md:max-w-none ${
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
        {/* Brand Header */}
        <div
          className="flex items-center justify-between px-4 py-4"
          style={{ borderBottom: "1px solid var(--border)" }}
        >
          <div className="flex items-center gap-2.5">
            <div
              className="flex h-8 w-8 items-center justify-center rounded-xl overflow-hidden shadow-sm border"
              style={{
                backgroundColor: "var(--surface)",
                borderColor: "var(--border)",
              }}
            >
              <svg
                width={16}
                height={16}
                viewBox="0 0 24 24"
                fill="none"
                stroke="var(--accent)"
                strokeWidth={1.8}
                strokeLinecap="round"
              >
                <path d="M12 3.5c.6 3.7 1.8 4.9 5.5 5.5-3.7.6-4.9 1.8-5.5 5.5-.6-3.7-1.8-4.9-5.5-5.5 3.7-.6 4.9-1.8 5.5-5.5Z" />
              </svg>
            </div>
            <div>
              <p
                className="font-display text-[13px] font-bold tracking-tight"
                style={{ color: "var(--text)" }}
              >
                J.A.R.V.I.S.
              </p>
              <p
                className="text-[9px] font-mono uppercase tracking-wider"
                style={{ color: "var(--text-tertiary)" }}
              >
                AI AGENT CORE
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            aria-label="Close menu"
            className="rounded-lg p-1.5 transition hover:bg-[var(--bg-hover)] md:hidden"
            style={{ color: "var(--text-secondary)" }}
          >
            <CloseIcon />
          </button>
        </div>

        {/* New Session Button */}
        <div className="px-3.5 pt-3.5">
          <Button
            variant="secondary"
            className="w-full justify-between"
            onClick={onNew}
            leftIcon={<PlusIcon width={13} height={13} />}
            rightIcon={<Kbd>⌘N</Kbd>}
          >
            New conversation
          </Button>
        </div>

        {/* Navigation Switcher Tabs */}
        <nav className="mt-3 grid grid-cols-2 gap-1 px-3.5">
          <NavTab
            active={view === "chat"}
            onClick={() => onViewChange("chat")}
            icon={<ChatIcon width={14} height={14} />}
          >
            Chat
          </NavTab>
          <NavTab
            active={view === "documents"}
            onClick={() => onViewChange("documents")}
            icon={<DocIcon width={14} height={14} />}
          >
            Documents
          </NavTab>
        </nav>

        {/* Main List Area */}
        <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
          {view === "documents" && (
            <div className="px-3.5 pt-3 pb-1">
              <div
                className="rounded-xl border p-3 text-xs shadow-sm transition"
                style={{
                  backgroundColor: "var(--bg-raised)",
                  borderColor: "var(--border)",
                }}
              >
                <div className="flex items-center gap-2 mb-1.5 font-semibold text-[var(--accent)]">
                  <DocIcon width={14} height={14} />
                  <span>RAG Knowledge Base</span>
                </div>
                <p className="text-[11px] leading-relaxed text-[var(--text-secondary)]">
                  Uploaded documents are vectorized and indexed for J.A.R.V.I.S. contextual retrieval.
                </p>
              </div>
            </div>
          )}

          <div className="flex items-center justify-between px-4 pb-1.5 pt-3">
            <p
              className="text-[10px] font-bold uppercase tracking-widest"
              style={{ color: "var(--text-tertiary)" }}
            >
              Recent Chats
            </p>
            <span className="text-[10px] font-mono text-[var(--text-tertiary)]">
              {conversations.length}
            </span>
          </div>

          {/* Search bar */}
          <div className="px-3.5 pb-2 shrink-0">
            <SearchBar
              value={searchQuery}
              onChange={setSearchQuery}
              placeholder="Search history..."
            />
          </div>

          <nav className="scroll-fine min-h-0 flex-1 space-y-1 overflow-y-auto px-3.5 pb-4">
            {loading ? (
              <div className="space-y-1 pt-1">
                {Array.from({ length: 5 }).map((_, i) => (
                  <ConversationSkeleton key={i} />
                ))}
              </div>
            ) : filtered.length === 0 ? (
              <p
                className="px-3 py-6 text-center text-[11px]"
                style={{ color: "var(--text-tertiary)" }}
              >
                {searchQuery ? "No matching conversations." : "No conversations yet."}
              </p>
            ) : (
              filtered.map((c) => {
                const active = c._id === activeId;
                if (renamingId === c._id) {
                  return (
                    <div
                      key={c._id}
                      className="flex items-center gap-2 rounded-xl px-3 py-1.5 border border-[var(--accent)]"
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
                        className="min-w-0 flex-1 bg-transparent text-[12px] outline-none font-medium"
                        style={{ color: "var(--text)" }}
                      />
                      <button
                        onClick={commitRename}
                        aria-label="Confirm"
                        className="shrink-0 rounded-md p-1 hover:bg-black/10"
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
                    className={`group/item relative flex w-full items-center gap-1.5 rounded-xl pr-1.5 transition-all duration-150 ${
                      active
                        ? "bg-[var(--accent-bg)] font-medium border border-[var(--accent)]/30"
                        : "hover:bg-[var(--bg-raised)]"
                    }`}
                  >
                    {active && (
                      <span className="absolute left-0 top-2 bottom-2 w-1 rounded-r-full bg-[var(--accent)]" />
                    )}

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
                      className="shrink-0 rounded-lg p-1 opacity-0 transition hover:bg-[var(--border)] focus:opacity-100 group-hover/item:opacity-100"
                      style={{ color: "var(--text-tertiary)" }}
                    >
                      <EditIcon width={12} height={12} />
                    </button>
                    <button
                      onClick={() => setPendingDelete(c)}
                      aria-label={`Delete`}
                      className="shrink-0 rounded-lg p-1 opacity-0 transition hover:bg-[var(--danger-bg)] hover:text-[var(--danger)] focus:opacity-100 group-hover/item:opacity-100"
                      style={{ color: "var(--text-tertiary)" }}
                    >
                      <TrashIcon width={13} height={13} />
                    </button>
                  </div>
                );
              })
            )}
          </nav>
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
};

export default Sidebar;
