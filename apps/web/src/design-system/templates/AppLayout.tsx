import { Suspense, type FC } from "react";
import { Outlet } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { Sidebar } from "../organisms/Sidebar";
import { Header } from "../organisms/Header";
import {
  ConversationProvider,
  useConversation,
} from "@/context/ConversationContext";
import { useToast } from "@/design-system/molecules/Toast";
import { getMessages } from "@/modules/chat/chat.api";

const LayoutContent: FC = () => {
  const { t } = useTranslation("layout");
  const { toggleSidebar, activeId } = useConversation();
  const toast = useToast();

  const handleExportChat = async () => {
    if (!activeId) {
      toast.error("Chưa chọn cuộc hội thoại để xuất");
      return;
    }
    try {
      const msgs = (await getMessages(activeId)) as Array<{
        role: string;
        content: string;
      }>;
      if (!msgs.length) {
        toast.error("Chưa có tin nhắn để xuất");
        return;
      }
      const content = msgs
        .map(
          (m) =>
            `### ${m.role === "user" ? "👤 Bạn" : "🤖 J.A.R.V.I.S."}\n\n${m.content}\n\n---`,
        )
        .join("\n\n");
      const blob = new Blob([content], { type: "text/markdown;charset=utf-8" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `javis-chat-${activeId}.md`;
      a.click();
      URL.revokeObjectURL(url);
      toast.success("Đã xuất cuộc trò chuyện ra file Markdown!");
    } catch {
      toast.error("Không thể tải tin nhắn để xuất file");
    }
  };

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
        <Header
          onToggleSidebar={toggleSidebar}
          onExportChat={activeId ? handleExportChat : undefined}
        />

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
                <span className="text-xs">
                  {t("appLayout.loadingApplication")}
                </span>
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
