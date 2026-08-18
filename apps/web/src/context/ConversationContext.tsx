/**
 * ConversationContext — giờ chỉ còn giữ CLIENT state của sidebar
 * (mở/đóng, thu gọn) và tính activeId/view từ URL.
 *
 * Danh sách hội thoại là server state nên đã chuyển sang TanStack Query
 * (useConversations). Trước đây context tự giữ useState + useEffect fetch,
 * nghĩa là mỗi lần ConversationProvider mount lại là một request
 * /api/conversations, và xoá/đổi tên luôn kèm một lần reload cả danh sách.
 *
 * API của context giữ nguyên để Sidebar/AppLayout/ChatPage không phải sửa gì.
 */

import {
  createContext,
  useContext,
  useState,
  useCallback,
  useEffect,
  type FC,
  type ReactNode,
} from "react";
import { useNavigate, useLocation } from "react-router-dom";
import {
  useConversations,
  type Conversation,
} from "@/hooks/queries/useConversations";
import { useMessagesCache } from "@/hooks/queries/useMessages";

interface ConversationContextType {
  conversations: Conversation[];
  loadingConversations: boolean;
  reloadConversations: () => Promise<void>;
  deleteConv: (id: string) => Promise<void>;
  renameConv: (id: string, title: string) => Promise<void>;
  sidebarOpen: boolean;
  collapsed: boolean;
  toggleSidebar: () => void;
  setSidebarOpen: (open: boolean) => void;
  activeId: string | null;
  view: "chat" | "documents";
}

const ConversationContext = createContext<ConversationContextType | null>(null);

export const ConversationProvider: FC<{ children: ReactNode }> = ({
  children,
}) => {
  const {
    conversations,
    loadingConversations,
    reloadConversations,
    deleteConv: deleteConvMutation,
    renameConv,
  } = useConversations();
  const { dropMessages } = useMessagesCache();

  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [collapsed, setCollapsed] = useState(
    () => localStorage.getItem("sb-collapsed") === "1",
  );
  const navigate = useNavigate();
  const location = useLocation();

  useEffect(() => {
    localStorage.setItem("sb-collapsed", collapsed ? "1" : "0");
  }, [collapsed]);

  const toggleSidebar = useCallback(() => {
    if (window.matchMedia("(min-width: 768px)").matches) {
      setCollapsed((c) => !c);
    } else {
      setSidebarOpen((o) => !o);
    }
  }, []);

  const activeId = location.pathname.startsWith("/messages/")
    ? (location.pathname.split("/")[2] ?? null)
    : null;

  const view = location.pathname.startsWith("/documents")
    ? "documents"
    : "chat";

  const deleteConv = useCallback(
    async (id: string) => {
      if (activeId === id) {
        navigate("/");
      }
      // Bỏ luôn cache tin nhắn của hội thoại vừa xoá — nếu không, tạo hội
      // thoại mới trùng id (hoặc quay lại bằng nút back) sẽ đọc được lịch sử
      // của một hội thoại không còn tồn tại.
      dropMessages(id);
      await deleteConvMutation(id);
    },
    [activeId, navigate, deleteConvMutation, dropMessages],
  );

  return (
    <ConversationContext.Provider
      value={{
        conversations,
        loadingConversations,
        reloadConversations: async () => {
          await reloadConversations();
        },
        deleteConv,
        renameConv: async (id, title) => {
          await renameConv(id, title);
        },
        sidebarOpen,
        collapsed,
        toggleSidebar,
        setSidebarOpen,
        activeId,
        view,
      }}
    >
      {children}
    </ConversationContext.Provider>
  );
};

export const useConversation = () => {
  const ctx = useContext(ConversationContext);
  if (!ctx) {
    throw new Error(
      "useConversation must be used within a ConversationProvider",
    );
  }
  return ctx;
};
