import { fireEvent, render, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";
import i18n from "@/i18n";
import { useUserStore } from "@/stores/user.store";
import { EmptyState } from "./EmptyState";

const renderEmptyState = () =>
  render(
    <I18nextProvider i18n={i18n}>
      <EmptyState onPick={() => {}} />
    </I18nextProvider>,
  );

describe("EmptyState", () => {
  beforeEach(async () => {
    await i18n.changeLanguage("vi");
  });

  it("shows Vietnamese header, subtitle and category tabs", async () => {
    renderEmptyState();

    expect(
      screen.getByText("Tôi có thể giúp gì cho bạn hôm nay?"),
    ).toBeInTheDocument();
    expect(screen.getByText("Ý tưởng & Kế hoạch")).toBeInTheDocument();
    expect(
      screen.getByTitle("Đổi bộ gợi ý ngẫu nhiên mới"),
    ).toBeInTheDocument();

    expect(
      await screen.findByText("Gợi ý thông minh từ AI Agent"),
    ).toBeInTheDocument();
  });

  it("shows English header, subtitle and category tabs", async () => {
    await i18n.changeLanguage("en");
    renderEmptyState();

    expect(screen.getByText("How can I help you today?")).toBeInTheDocument();
    expect(screen.getByText("Ideas & Planning")).toBeInTheDocument();
    expect(screen.getByTitle("Refresh random suggestions")).toBeInTheDocument();

    expect(
      await screen.findByText("Smart Suggestions from AI Agent"),
    ).toBeInTheDocument();
  });

  // User upload avatar cho agent nhưng khung chat không đổi gì — vì lúc chưa có
  // tin nhắn nào thì MessageBubble chưa render, mà hero của EmptyState lại
  // hardcode icon. Nhóm test này khoá lại hành vi đó.
  describe("avatar agent ở hero", () => {
    beforeEach(() => {
      useUserStore.setState({ settings: null });
    });

    it("hiện icon mặc định khi user chưa cấu hình avatar agent", () => {
      renderEmptyState();
      expect(screen.queryByRole("img")).not.toBeInTheDocument();
    });

    it("hiện ảnh thật khi settings có agent_avatar_url", () => {
      useUserStore.setState({
        settings: {
          agent_avatar_url: "https://cdn.example.com/agent.png",
        } as never,
      });

      renderEmptyState();

      const img = screen.getByRole("img");
      expect(img).toHaveAttribute("src", "https://cdn.example.com/agent.png");
    });

    it("ảnh lỗi 404 thì fallback về icon, không để khung trống", () => {
      useUserStore.setState({
        settings: {
          agent_avatar_url: "https://cdn.example.com/hong.png",
        } as never,
      });

      renderEmptyState();
      fireEvent.error(screen.getByRole("img"));

      expect(screen.queryByRole("img")).not.toBeInTheDocument();
    });
  });
});
