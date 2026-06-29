import { useCallback, useEffect, useState } from "react";
import { Outlet, useLocation, useNavigate } from "react-router-dom";
import Sidebar from "./Sidebar";
import { listConversations, type Conversation } from "../lib/api";

// Dữ liệu chia sẻ xuống các page con qua Outlet context
export type OutletCtx = {
  conversations: Conversation[];
  reloadConversations: () => Promise<void>;
  openSidebar: () => void;
};

export default function AppLayout() {
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const navigate = useNavigate();
  const location = useLocation();

  const reloadConversations = useCallback(async () => {
    setConversations(await listConversations());
  }, []);

  useEffect(() => {
    reloadConversations();
  }, [reloadConversations]);

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
      />

      <Outlet
        context={
          {
            conversations,
            reloadConversations,
            openSidebar: () => setSidebarOpen(true),
          } satisfies OutletCtx
        }
      />
    </div>
  );
}
