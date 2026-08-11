import { Suspense, useCallback, useEffect, useState } from "react";
import { Outlet, useLocation, useNavigate } from "react-router-dom";
import { Sidebar } from "../organisms/Sidebar";
import { Header } from "../organisms/Header";
import {
  listConversations,
  deleteConversation,
  renameConversation,
  type Conversation,
} from "@/modules/chat/chat.api";

export type OutletCtx = {
  conversations: Conversation[];
  loadingConversations: boolean;
  reloadConversations: () => Promise<void>;
  toggleSidebar: () => void;
};

/**
 * AppLayout template component providing main application shell, sidebar drawer,
 * top header, and nested route Outlet container.
 */
export const AppLayout: React.FC = () => {
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [loadingConversations, setLoadingConversations] = useState(true);
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [collapsed, setCollapsed] = useState(
    () => localStorage.getItem("sb-collapsed") === "1",
  );
  const navigate = useNavigate();
  const location = useLocation();

  const reloadConversations = useCallback(async () => {
    try {
      setConversations(await listConversations());
    } catch {
      // Silently fail
    } finally {
      setLoadingConversations(false);
    }
  }, []);

  useEffect(() => {
    reloadConversations();
  }, [reloadConversations]);

  useEffect(() => {
    localStorage.setItem("sb-collapsed", collapsed ? "1" : "0");
  }, [collapsed]);

  const toggleSidebar = () => {
    if (window.matchMedia("(min-width: 768px)").matches) {
      setCollapsed((c) => !c);
    } else {
      setSidebarOpen((o) => !o);
    }
  };

  const activeId = location.pathname.startsWith("/messages/")
    ? (location.pathname.split("/")[2] ?? null)
    : null;
  const view = location.pathname.startsWith("/documents")
    ? "documents"
    : "chat";

  const handleDelete = async (id: string) => {
    await deleteConversation(id);
    await reloadConversations();
    if (activeId === id) navigate("/");
  };

  const handleRename = async (id: string, title: string) => {
    await renameConversation(id, title);
    await reloadConversations();
  };

  return (
    <div
      className="flex h-screen overflow-hidden relative"
      style={{ backgroundColor: "var(--bg)" }}
    >
      <Sidebar
        conversations={conversations}
        loading={loadingConversations}
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
        onDelete={handleDelete}
        onRename={handleRename}
      />

      {/* Main Content Area */}
      <div
        className="flex min-w-0 flex-1 flex-col relative z-10"
        style={{ minHeight: 0 }}
      >
        <Header onToggleSidebar={toggleSidebar} />

        <Suspense
          fallback={
            <main
              className="flex flex-1 items-center justify-center"
              style={{ minHeight: 0, color: "var(--text-tertiary)" }}
            >
              <div className="flex flex-col items-center gap-2">
                <div
                  className="h-6 w-6 animate-spin rounded-full border-2 border-t-transparent"
                  style={{ borderColor: "var(--accent)" }}
                />
                <span className="text-xs">Loading application...</span>
              </div>
            </main>
          }
        >
          <Outlet
            context={
              {
                conversations,
                loadingConversations,
                reloadConversations,
                toggleSidebar,
              } satisfies OutletCtx
            }
          />
        </Suspense>
      </div>
    </div>
  );
};

export default AppLayout;
