import { useCallback, useEffect, useState } from "react";
import { Outlet, useLocation, useNavigate } from "react-router-dom";
import Sidebar from "./Sidebar";
import {
  listConversations,
  deleteConversation,
  type Conversation,
} from "@/modules/chat/chat.api";

// Dữ liệu chia sẻ xuống các page con qua Outlet context
export type OutletCtx = {
  conversations: Conversation[];
  reloadConversations: () => Promise<void>;
  toggleSidebar: () => void;
};

export default function AppLayout() {
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [sidebarOpen, setSidebarOpen] = useState(false); // drawer trên mobile
  const [collapsed, setCollapsed] = useState(
    () => localStorage.getItem("sb-collapsed") === "1",
  ); // thu gọn trên desktop (nhớ qua reload)
  const navigate = useNavigate();
  const location = useLocation();

  const reloadConversations = useCallback(async () => {
    setConversations(await listConversations());
  }, []);

  useEffect(() => {
    reloadConversations();
  }, [reloadConversations]);

  useEffect(() => {
    localStorage.setItem("sb-collapsed", collapsed ? "1" : "0");
  }, [collapsed]);

  // Cùng 1 nút: desktop → thu gọn/mở; mobile → bật/tắt drawer
  const toggleSidebar = () => {
    if (window.matchMedia("(min-width: 768px)").matches) {
      setCollapsed((c) => !c);
    } else {
      setSidebarOpen((o) => !o);
    }
  };

  // Trạng thái active suy ra TỪ URL → reload trang vẫn đúng phiên
  const activeId = location.pathname.startsWith("/messages/")
    ? (location.pathname.split("/")[2] ?? null)
    : null;
  const view = location.pathname.startsWith("/documents") ? "documents" : "chat";

  return (
    <div className="flex h-screen overflow-hidden bg-surface">
      <Sidebar
        conversations={conversations}
        activeId={activeId}
        open={sidebarOpen}
        collapsed={collapsed}
        view={view}
        onSelect={(id) => {
          navigate(`/messages/${id}`);
          setSidebarOpen(false);
        }}
        onNew={() => {
          navigate("/");
          setSidebarOpen(false);
        }}
        onClose={() => setSidebarOpen(false)}
        onViewChange={(v) => {
          navigate(v === "documents" ? "/documents" : "/");
          setSidebarOpen(false);
        }}
        onDelete={async (id) => {
          await deleteConversation(id);
          await reloadConversations();
          // nếu đang mở hội thoại vừa xóa → về home
          if (activeId === id) navigate("/");
        }}
      />

      <Outlet
        context={
          {
            conversations,
            reloadConversations,
            toggleSidebar,
          } satisfies OutletCtx
        }
      />
    </div>
  );
}
