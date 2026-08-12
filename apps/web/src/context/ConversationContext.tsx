import {
  createContext,
  useContext,
  useState,
  useCallback,
  useEffect,
  type FC,
  type ReactNode,
} from "react";
import {
  listConversations,
  deleteConversation,
  renameConversation,
  type Conversation,
} from "@/modules/chat/chat.api";
import { useNavigate, useLocation } from "react-router-dom";

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
      // Ignore failure
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
      setConversations((prev) => prev.filter((c) => c._id !== id));
      try {
        await deleteConversation(id);
      } catch {
        // Ignore
      } finally {
        reloadConversations();
      }
    },
    [activeId, navigate, reloadConversations],
  );

  const renameConv = useCallback(
    async (id: string, title: string) => {
      setConversations((prev) =>
        prev.map((c) => (c._id === id ? { ...c, title } : c)),
      );
      try {
        await renameConversation(id, title);
      } catch {
        // Ignore
      } finally {
        reloadConversations();
      }
    },
    [reloadConversations],
  );

  return (
    <ConversationContext.Provider
      value={{
        conversations,
        loadingConversations,
        reloadConversations,
        deleteConv,
        renameConv,
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
