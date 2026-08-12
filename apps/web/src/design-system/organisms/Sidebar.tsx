import { useState, useMemo } from "react";
import {
  PlusIcon,
  ChatBubbleLeftRightIcon,
  DocumentTextIcon,
  XMarkIcon,
  TrashIcon,
  PencilSquareIcon,
  CheckIcon,
  MagnifyingGlassIcon,
  ArrowRightOnRectangleIcon,
  SparklesIcon,
} from "@heroicons/react/24/outline";

import { useAuthStore } from "@/stores/auth.store";
import { useToast } from "@/shared/components/Toast";
import ConfirmDialog from "@/shared/components/ConfirmDialog";
import type { Conversation } from "@/modules/chat/chat.api";
import { useConversation } from "@/context/ConversationContext";
import { useNavigate } from "react-router-dom";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Skeleton } from "@/components/ui/skeleton";

export type View = "chat" | "documents";

export interface SidebarProps {
  conversations?: Conversation[];
  loading?: boolean;
  activeId?: string | null;
  open?: boolean;
  collapsed?: boolean;
  view?: View;
  onSelect?: (id: string) => void;
  onNew?: () => void;
  onClose?: () => void;
  onViewChange?: (v: View) => void;
  onDelete?: (id: string) => void;
  onRename?: (id: string, title: string) => void;
}

const ConversationSkeleton: React.FC = () => (
  <div className="flex items-center gap-2.5 px-3 py-2">
    <Skeleton className="h-4 w-4 shrink-0 rounded-md bg-muted/60" />
    <Skeleton className="h-3.5 flex-1 rounded-md bg-muted/50" />
  </div>
);

function groupConversationsByDate(convs: Conversation[]) {
  const now = new Date();
  const todayStart = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
  const yesterdayStart = todayStart - 86400000;
  const weekStart = todayStart - 6 * 86400000;

  const groups: { label: string; items: Conversation[] }[] = [
    { label: "Hôm nay", items: [] },
    { label: "Hôm qua", items: [] },
    { label: "7 ngày qua", items: [] },
    { label: "Cũ hơn", items: [] },
  ];

  convs.forEach((c) => {
    const time = c.createdAt ? new Date(c.createdAt).getTime() : now.getTime();
    if (time >= todayStart) {
      groups[0].items.push(c);
    } else if (time >= yesterdayStart) {
      groups[1].items.push(c);
    } else if (time >= weekStart) {
      groups[2].items.push(c);
    } else {
      groups[3].items.push(c);
    }
  });

  return groups.filter((g) => g.items.length > 0);
}

/**
 * Sidebar — Shadcn UI styled navigation drawer with Heroicons and User Profile footer.
 */
export const Sidebar: React.FC<SidebarProps> = (props) => {
  const ctx = useConversation();
  const navigate = useNavigate();

  const conversations = props.conversations ?? ctx.conversations;
  const loading = props.loading ?? ctx.loadingConversations;
  const activeId = props.activeId ?? ctx.activeId;
  const open = props.open ?? ctx.sidebarOpen;
  const collapsed = props.collapsed ?? ctx.collapsed;
  const view = props.view ?? ctx.view;

  const onSelect = props.onSelect ?? ((id) => { navigate(`/messages/${id}`); ctx.setSidebarOpen(false); });
  const onNew = props.onNew ?? (() => { navigate("/"); ctx.setSidebarOpen(false); });
  const onClose = props.onClose ?? (() => ctx.setSidebarOpen(false));
  const onViewChange = props.onViewChange ?? ((v) => { navigate(v === "documents" ? "/documents" : "/"); ctx.setSidebarOpen(false); });
  const onDelete = props.onDelete ?? ctx.deleteConv;
  const onRename = props.onRename ?? ctx.renameConv;

  const { user, logout } = useAuthStore();
  const toast = useToast();
  const [pendingDelete, setPendingDelete] = useState<Conversation | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [renamingId, setRenamingId] = useState<string | null>(null);
  const [renameValue, setRenameValue] = useState("");

  const filtered = useMemo(() => {
    if (!searchQuery.trim()) return conversations;
    const q = searchQuery.toLowerCase();
    return conversations.filter((c) => c.title.toLowerCase().includes(q));
  }, [conversations, searchQuery]);

  const grouped = useMemo(() => {
    return groupConversationsByDate(filtered);
  }, [filtered]);

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

  const handleLogout = async () => {
    try {
      await logout();
      toast.success("Đã đăng xuất thành công!");
    } catch {
      toast.error("Đã xảy ra lỗi khi đăng xuất.");
    }
  };

  const userInitials = user?.name
    ? user.name
        .split(" ")
        .map((n) => n[0])
        .join("")
        .toUpperCase()
        .slice(0, 2)
    : "AI";

  return (
    <>
      {/* Mobile overlay */}
      {open && (
        <div
          className="fixed inset-0 z-20 bg-black/50 backdrop-blur-sm md:hidden"
          onClick={onClose}
          aria-hidden
        />
      )}

      <aside
        className={`fixed inset-y-0 left-0 z-30 flex w-[85vw] max-w-[280px] flex-col transition-all duration-300 ease-out md:static md:max-w-none bg-card/85 border-r border-border backdrop-blur-2xl ${
          open ? "translate-x-0" : "-translate-x-full"
        } ${
          collapsed
            ? "md:w-0 md:-translate-x-full md:overflow-hidden"
            : "md:w-[280px] md:translate-x-0"
        }`}
      >
        {/* Brand */}
        <div className="flex items-center justify-between px-4 py-3.5 border-b border-border">
          <div className="flex items-center gap-2.5">
            <div className="flex h-8 w-8 items-center justify-center rounded-xl bg-gradient-to-br from-indigo-500 to-purple-600 text-white shadow-sm">
              <SparklesIcon className="h-4 w-4" />
            </div>
            <div>
              <p className="font-display text-[13px] font-bold tracking-tight text-foreground">
                J.A.R.V.I.S.
              </p>
              <p className="text-[9px] font-mono uppercase tracking-wider text-primary font-bold">
                SMART AGENT CORE
              </p>
            </div>
          </div>
          <Button
            variant="ghost"
            size="iconSm"
            onClick={onClose}
            aria-label="Close menu"
            className="md:hidden"
          >
            <XMarkIcon className="h-4 w-4" />
          </Button>
        </div>

        {/* New Session Button */}
        <div className="px-3.5 pt-3.5">
          <Button
            variant="gradient"
            className="w-full justify-start gap-2 text-xs font-bold"
            onClick={onNew}
          >
            <PlusIcon className="h-4 w-4" />
            <span>Tạo cuộc trò chuyện mới</span>
          </Button>
        </div>

        {/* Navigation Tabs */}
        <nav className="mt-3 grid grid-cols-2 gap-1 px-3.5">
          <Button
            type="button"
            variant={view === "chat" ? "secondary" : "ghost"}
            size="sm"
            onClick={() => onViewChange("chat")}
            className="gap-2 justify-center"
          >
            <ChatBubbleLeftRightIcon className="h-3.5 w-3.5 text-primary" />
            <span>Hội thoại</span>
          </Button>
          <Button
            type="button"
            variant={view === "documents" ? "secondary" : "ghost"}
            size="sm"
            onClick={() => onViewChange("documents")}
            className="gap-2 justify-center"
          >
            <DocumentTextIcon className="h-3.5 w-3.5 text-primary" />
            <span>Tài liệu</span>
          </Button>
        </nav>

        {/* Content Area */}
        <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
          {view === "documents" && (
            <div className="px-3.5 pt-3 pb-1">
              <div className="rounded-xl border border-primary/20 bg-primary/5 p-3 text-xs">
                <div className="flex items-center gap-2 mb-1 font-semibold text-primary">
                  <DocumentTextIcon className="h-4 w-4" />
                  <span>RAG Knowledge Base</span>
                </div>
                <p className="text-[11px] leading-relaxed text-muted-foreground">
                  Tài liệu tải lên được tự động phân tách & vectorized cho AI truy vấn.
              </div>
            </div>
          )}

          {/* Search Filter Bar */}
          <div className="px-3.5 pt-3 pb-2 shrink-0 relative">
            <div className="relative">
              <MagnifyingGlassIcon className="absolute left-3 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
              <Input
                type="text"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder="Tìm kiếm hội thoại..."
                className="pl-8 h-8 text-xs bg-muted/40"
              />
            </div>
          </div>

          {/* Grouped Conversation List */}
          <nav className="scroll-fine min-h-0 flex-1 space-y-3 overflow-y-auto px-3.5 pb-4">
            {loading ? (
              <div className="space-y-1 pt-1">
                {Array.from({ length: 5 }).map((_, i) => (
                  <ConversationSkeleton key={i} />
                ))}
              </div>
            ) : filtered.length === 0 ? (
              <p className="px-3 py-6 text-center text-[11px] text-muted-foreground">
                {searchQuery
                  ? "Không tìm thấy hội thoại phù hợp."
                  : "Chưa có cuộc trò chuyện nào."}
            ) : (
              grouped.map((group) => (
                <div key={group.label} className="space-y-0.5">
                  <div className="px-2 py-1 flex items-center justify-between">
                    <span className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
                      {group.label}
                    </span>
                    <Badge variant="outline" className="text-[9px] px-1.5 py-0 font-mono">
                      {group.items.length}
                    </Badge>
                  </div>

                  {group.items.map((c) => {
                    const active = c._id === activeId;
                    if (renamingId === c._id) {
                      return (
                        <div
                          key={c._id}
                          className="flex items-center gap-2 rounded-xl px-3 py-1.5 bg-primary/10 border border-primary/30"
                        >
                          <ChatBubbleLeftRightIcon className="h-4 w-4 text-primary shrink-0" />
                          <Input
                            type="text"
                            value={renameValue}
                            onChange={(e) => setRenameValue(e.target.value)}
                            onKeyDown={handleRenameKeyDown}
                            onBlur={commitRename}
                            autoFocus
                            className="h-7 text-xs bg-transparent border-0 px-0 outline-none focus-visible:ring-0"
                          />
                          <button
                            onClick={commitRename}
                            aria-label="Xác nhận đổi tên"
                            className="shrink-0 text-primary hover:opacity-80"
                          >
                            <CheckIcon className="h-4 w-4" />
                          </button>
                        </div>
                      );
                    }
                    return (
                      <div
                        key={c._id}
                        className={`group/item relative flex w-full items-center gap-1.5 rounded-xl pr-1.5 transition-all duration-150 ${
                          active
                            ? "bg-primary/10 text-primary font-semibold"
                            : "hover:bg-muted/60 text-foreground"
                        }`}
                      >
                        {active && (
                          <span className="absolute left-0 top-2 bottom-2 w-0.5 rounded-r-full bg-primary" />
                        )}
                        <button
                          onClick={() => onSelect(c._id)}
                          className="flex min-w-0 flex-1 items-center gap-2.5 py-2 pl-3 text-left"
                        >
                          <ChatBubbleLeftRightIcon
                            className={`h-3.5 w-3.5 shrink-0 ${
                              active ? "text-primary" : "text-muted-foreground"
                            }`}
                          />
                          <span className="truncate text-sm">{c.title}</span>
                        </button>
                        <button
                          onClick={() => startRename(c)}
                          aria-label="Đổi tên"
                          className="shrink-0 p-1 opacity-0 transition hover:bg-muted rounded-md focus:opacity-100 group-hover/item:opacity-100 text-muted-foreground"
                        >
                          <PencilSquareIcon className="h-3.5 w-3.5" />
                        </button>
                        <button
                          onClick={() => setPendingDelete(c)}
                          aria-label="Xóa cuộc trò chuyện"
                          className="shrink-0 p-1 opacity-0 transition hover:bg-destructive/10 hover:text-destructive rounded-md focus:opacity-100 group-hover/item:opacity-100 text-muted-foreground"
                        >
                          <TrashIcon className="h-3.5 w-3.5" />
                        </button>
                      </div>
                    );
                  })}
                </div>
              ))
            )}
          </nav>
        </div>

        {/* ── User Profile & Logout Footer ── */}
        <div className="p-3 shrink-0 border-t border-border bg-card/40">
          <div className="flex items-center justify-between gap-2 rounded-xl p-2 bg-background border border-border">
            <div className="flex items-center gap-2.5 min-w-0 flex-1">
              <Avatar className="h-8 w-8">
                <AvatarFallback className="bg-gradient-to-br from-indigo-500 to-purple-600 text-white font-bold text-xs">
                  {userInitials}
                </AvatarFallback>
              </Avatar>
              <div className="min-w-0 flex-1">
                <p className="truncate text-xs font-bold text-foreground">
                  {user?.name || "Người dùng"}
                </p>
                <p className="truncate text-[10px] text-muted-foreground">
                  {user?.email || "user@javis.ai"}
                </p>
              </div>
            </div>

            <Button
              type="button"
              variant="ghost"
              size="iconSm"
              onClick={handleLogout}
              aria-label="Đăng xuất"
              title="Đăng xuất tài khoản"
              className="text-muted-foreground hover:text-destructive hover:bg-destructive/10 h-8 w-8"
            >
              <ArrowRightOnRectangleIcon className="h-4 w-4" />
            </Button>
          </div>
        </div>
      </aside>

      <ConfirmDialog
        open={pendingDelete !== null}
        title="Xóa cuộc trò chuyện?"
        message={
          pendingDelete
            ? `Cuộc hội thoại "${pendingDelete.title}" sẽ bị xóa vĩnh viễn.`
            : undefined
        }
        confirmLabel="Xóa vĩnh viễn"
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
