import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi, type Mock } from "vitest";
import i18n from "@/i18n";
import type { Conversation } from "@/modules/chat/chat.api";
import { Sidebar } from "./Sidebar";

vi.mock("@/context/ConversationContext", () => ({
  useConversation: vi.fn(),
}));
vi.mock("@/stores/auth.store", () => ({
  useAuthStore: vi.fn(),
}));
vi.mock("@/design-system/molecules/Toast", () => ({
  useToast: vi.fn(),
}));

import { useConversation } from "@/context/ConversationContext";
import { useAuthStore } from "@/stores/auth.store";
import { useToast } from "@/design-system/molecules/Toast";

const mockUseConversation = useConversation as unknown as Mock;
const mockUseAuthStore = useAuthStore as unknown as Mock;
const mockUseToast = useToast as unknown as Mock;

const conversations: Conversation[] = [
  { _id: "c1", title: "Alpha", createdAt: new Date().toISOString() },
];

const baseCtx = {
  conversations,
  loadingConversations: false,
  activeId: null,
  sidebarOpen: true,
  collapsed: false,
  view: "chat" as const,
  setSidebarOpen: vi.fn(),
  deleteConv: vi.fn(),
  renameConv: vi.fn(),
  toggleSidebar: vi.fn(),
  reloadConversations: vi.fn(),
};

const toastApi = {
  success: vi.fn(),
  error: vi.fn(),
  info: vi.fn(),
  warning: vi.fn(),
};

const renderSidebar = () =>
  render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter>
        <Sidebar />
      </MemoryRouter>
    </I18nextProvider>,
  );

describe("Sidebar", () => {
  beforeEach(async () => {
    vi.clearAllMocks();
    mockUseConversation.mockReturnValue(baseCtx);
    mockUseAuthStore.mockReturnValue({
      user: { name: "Trinh", email: "t@x.com" },
      logout: vi.fn().mockResolvedValue(undefined),
    });
    mockUseToast.mockReturnValue(toastApi);
    await i18n.changeLanguage("vi");
  });

  it("renders Vietnamese labels for the primary layout", () => {
    renderSidebar();

    expect(screen.getByText("Tạo cuộc trò chuyện mới")).toBeInTheDocument();
    expect(screen.getByText("Hội thoại")).toBeInTheDocument();
    expect(screen.getByText("Tài liệu")).toBeInTheDocument();
    expect(
      screen.getByPlaceholderText("Tìm kiếm hội thoại..."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Đóng menu" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Đổi tên" })).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Xóa cuộc trò chuyện" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Đăng xuất" })).toHaveAttribute(
      "title",
      "Đăng xuất tài khoản",
    );
    expect(screen.getByText("Hôm nay")).toBeInTheDocument();
  });

  it("renders English labels for the primary layout", async () => {
    await i18n.changeLanguage("en");
    renderSidebar();

    expect(screen.getByText("New conversation")).toBeInTheDocument();
    expect(screen.getByText("Chats")).toBeInTheDocument();
    expect(screen.getByText("Documents")).toBeInTheDocument();
    expect(
      screen.getByPlaceholderText("Search conversations..."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Close menu" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Rename" })).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Delete conversation" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Log out" })).toHaveAttribute(
      "title",
      "Log out of account",
    );
    expect(screen.getByText("Today")).toBeInTheDocument();
  });

  it("shows the translated empty state when there are no conversations", () => {
    mockUseConversation.mockReturnValue({ ...baseCtx, conversations: [] });
    renderSidebar();
    expect(
      screen.getByText("Chưa có cuộc trò chuyện nào."),
    ).toBeInTheDocument();
  });

  it("shows the translated no-results message when a search matches nothing", async () => {
    const user = userEvent.setup();
    renderSidebar();

    await user.type(
      screen.getByPlaceholderText("Tìm kiếm hội thoại..."),
      "zzz-no-match",
    );

    expect(
      screen.getByText("Không tìm thấy hội thoại phù hợp."),
    ).toBeInTheDocument();
  });

  it("shows a translated default user name when no user is present", () => {
    mockUseAuthStore.mockReturnValue({ user: null, logout: vi.fn() });
    renderSidebar();
    expect(screen.getByText("Người dùng")).toBeInTheDocument();
  });

  it("shows the translated knowledge base hint in the documents view", () => {
    mockUseConversation.mockReturnValue({ ...baseCtx, view: "documents" });
    renderSidebar();
    expect(screen.getByText("RAG Knowledge Base")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Tài liệu tải lên được tự động phân tách & vectorized cho AI truy vấn.",
      ),
    ).toBeInTheDocument();
  });

  it("shows the translated delete confirmation dialog with the interpolated conversation title", async () => {
    const user = userEvent.setup();
    renderSidebar();

    await user.click(
      screen.getByRole("button", { name: "Xóa cuộc trò chuyện" }),
    );

    expect(await screen.findByText("Xóa cuộc trò chuyện?")).toBeInTheDocument();
    expect(
      screen.getByText('Cuộc hội thoại "Alpha" sẽ bị xóa vĩnh viễn.'),
    ).toBeInTheDocument();
    expect(screen.getByText("Xóa vĩnh viễn")).toBeInTheDocument();
  });

  it("shows the translated rename confirmation button after starting a rename", async () => {
    const user = userEvent.setup();
    renderSidebar();

    await user.click(screen.getByRole("button", { name: "Đổi tên" }));

    expect(
      screen.getByRole("button", { name: "Xác nhận đổi tên" }),
    ).toBeInTheDocument();
  });

  it("shows a translated success toast after logout", async () => {
    const logout = vi.fn().mockResolvedValue(undefined);
    mockUseAuthStore.mockReturnValue({
      user: { name: "Trinh", email: "t@x.com" },
      logout,
    });
    const user = userEvent.setup();
    renderSidebar();

    await user.click(screen.getByRole("button", { name: "Đăng xuất" }));

    await waitFor(() =>
      expect(toastApi.success).toHaveBeenCalledWith("Đã đăng xuất thành công!"),
    );
  });

  it("shows a translated error toast when logout fails", async () => {
    const logout = vi.fn().mockRejectedValue(new Error("boom"));
    mockUseAuthStore.mockReturnValue({
      user: { name: "Trinh", email: "t@x.com" },
      logout,
    });
    const user = userEvent.setup();
    renderSidebar();

    await user.click(screen.getByRole("button", { name: "Đăng xuất" }));

    await waitFor(() =>
      expect(toastApi.error).toHaveBeenCalledWith(
        "Đã xảy ra lỗi khi đăng xuất.",
      ),
    );
  });
});
