import type { Conversation } from "@app/types";

export type ViewMode = "chat" | "documents";

/** Outlet Context shared with child pages. */
export type OutletCtx = {
  conversations: Conversation[];
  loadingConversations: boolean;
  reloadConversations: () => Promise<void>;
  toggleSidebar: () => void;
};

/** Props for AppHeader organism component. */
export interface HeaderProps {
  onToggleSidebar: () => void;
  title?: string;
}

/** Props for Sidebar drawer organism component. */
export interface SidebarProps {
  conversations: Conversation[];
  loading: boolean;
  activeId: string | null;
  open: boolean;
  collapsed: boolean;
  view: ViewMode;
  onSelect: (id: string) => void;
  onNew: () => void;
  onClose: () => void;
  onViewChange: (v: ViewMode) => void;
  onDelete: (id: string) => void;
  onRename: (id: string, title: string) => void;
}
