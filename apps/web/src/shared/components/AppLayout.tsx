import { Suspense, useCallback, useEffect, useState } from "react";
import { Outlet, useLocation, useNavigate } from "react-router-dom";
import Sidebar from "./Sidebar";
import ThemeToggle from "./ThemeToggle";
import {
  listConversations,
  deleteConversation,
  renameConversation,
  type Conversation,
} from "@/modules/chat/chat.api";

// Data shared with child pages via Outlet context
export type OutletCtx = {
  conversations: Conversation[];
  loadingConversations: boolean;
  reloadConversations: () => Promise<void>;
  toggleSidebar: () => void;
};

export default function AppLayout() {
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
    <div className="flex h-screen overflow-hidden" style={{ backgroundColor: "var(--cyber-bg)" }}>
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

      {/* Main content */}
      <div className="flex min-w-0 flex-1 flex-col" style={{ minHeight: 0 }}>
        {/* Top bar */}
        <header
          className="flex shrink-0 items-center gap-3 border-b px-4 py-2.5 sm:px-6"
          style={{
            borderColor: "var(--cyber-border)",
            backgroundColor: "var(--cyber-surface)",
          }}
        >
          <button
            type="button"
            onClick={toggleSidebar}
            aria-label="Toggle sidebar"
            className="rounded-full p-2 transition hover:bg-[var(--cyber-subtle2)]"
            style={{ color: "var(--cyber-muted)" }}
          >
            <svg
              width={20}
              height={20}
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth={1.5}
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <path d="M4 6h16M4 12h16M4 18h16" />
            </svg>
          </button>

          <h1 className="text-sm font-medium tracking-wider" style={{ color: "var(--cyber-text)" }}>
            J.A.R.V.I.S.
          </h1>

          <div className="ml-auto">
            <ThemeToggle />
          </div>
        </header>

        {/* Page content */}
        <Suspense
          fallback={
            <main
              className="flex flex-1 items-center justify-center"
              style={{ minHeight: 0, color: "var(--cyber-faint)" }}
            >
              Loading...
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
}
