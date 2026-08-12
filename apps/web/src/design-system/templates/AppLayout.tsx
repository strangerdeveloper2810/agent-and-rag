import { Suspense, type FC } from "react";
import { Outlet } from "react-router-dom";
import { Sidebar } from "../organisms/Sidebar";
import { Header } from "../organisms/Header";
import {
  ConversationProvider,
  useConversation,
} from "@/context/ConversationContext";

const LayoutContent: FC = () => {
  const { toggleSidebar } = useConversation();

  return (
    <div
      className="flex h-screen overflow-hidden relative"
      style={{ backgroundColor: "var(--bg)" }}
    >
      <Sidebar />

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
          <Outlet />
        </Suspense>
      </div>
    </div>
  );
};

/**
 * AppLayout template component providing main application shell, sidebar drawer,
 * top header, and nested route Outlet container.
 */
export const AppLayout: FC = () => {
  return (
    <ConversationProvider>
      <LayoutContent />
    </ConversationProvider>
  );
};

export default AppLayout;
