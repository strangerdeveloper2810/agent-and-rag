import { lazy } from "react";
import { render, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "@/i18n";
import { withQueryClient } from "@/test/query";
import { AppLayout } from "./AppLayout";

vi.mock("../organisms/Sidebar", () => ({
  Sidebar: () => null,
  default: () => null,
}));
vi.mock("../organisms/Header", () => ({
  Header: () => null,
  default: () => null,
}));
vi.mock("@/modules/chat/chat.api", () => ({
  listConversations: vi.fn().mockResolvedValue([]),
  deleteConversation: vi.fn(),
  renameConversation: vi.fn(),
}));

// A lazy component whose import promise never resolves, so the Suspense
// fallback stays visible for the duration of the assertion.
const NeverResolves = lazy(() => new Promise<never>(() => {}));

// AppLayout dựng ConversationProvider, mà provider này lấy danh sách hội thoại
// qua TanStack Query → phải có QueryClientProvider bọc ngoài.
const renderLayout = () =>
  render(
    withQueryClient(
      <I18nextProvider i18n={i18n}>
        <MemoryRouter initialEntries={["/"]}>
          <Routes>
            <Route element={<AppLayout />}>
              <Route path="/" element={<NeverResolves />} />
            </Route>
          </Routes>
        </MemoryRouter>
      </I18nextProvider>,
    ),
  );

describe("AppLayout", () => {
  beforeEach(async () => {
    await i18n.changeLanguage("vi");
  });

  it("shows the Vietnamese suspense fallback text", () => {
    renderLayout();
    expect(screen.getByText("Đang tải ứng dụng...")).toBeInTheDocument();
  });

  it("shows the English suspense fallback text", async () => {
    await i18n.changeLanguage("en");
    renderLayout();
    expect(screen.getByText("Loading application...")).toBeInTheDocument();
  });
});
