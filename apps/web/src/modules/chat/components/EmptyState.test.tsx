import { render, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";
import i18n from "@/i18n";
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
});
