import { render, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "@/i18n";
import { Header } from "./Header";

const renderHeader = () =>
  render(
    <I18nextProvider i18n={i18n}>
      <Header
        onToggleSidebar={vi.fn()}
        onToggleSearch={vi.fn()}
        onExportChat={vi.fn()}
        onClearChat={vi.fn()}
      />
    </I18nextProvider>,
  );

describe("Header", () => {
  beforeEach(async () => {
    await i18n.changeLanguage("vi");
  });

  it("renders Vietnamese labels for all header controls", () => {
    renderHeader();

    expect(
      screen.getByRole("button", { name: "Chuyển đổi thanh bên" }),
    ).toBeInTheDocument();
    expect(screen.getByText("TRỰC TUYẾN")).toBeInTheDocument();

    const searchBtn = screen.getByRole("button", {
      name: "Tìm kiếm trong cuộc hội thoại",
    });
    expect(searchBtn).toHaveAttribute("title", "Tìm kiếm trong cuộc hội thoại");

    const exportBtn = screen.getByRole("button", {
      name: "Xuất lịch sử chat (Markdown/JSON)",
    });
    expect(exportBtn).toHaveAttribute(
      "title",
      "Xuất lịch sử chat (Markdown/JSON)",
    );

    const clearBtn = screen.getByRole("button", {
      name: "Làm sạch cuộc hội thoại",
    });
    expect(clearBtn).toHaveAttribute("title", "Làm sạch cuộc hội thoại");
  });

  it("renders English labels when locale is en", async () => {
    await i18n.changeLanguage("en");
    renderHeader();

    expect(
      screen.getByRole("button", { name: "Toggle sidebar" }),
    ).toBeInTheDocument();
    expect(screen.getByText("ONLINE")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Search conversation" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Export chat log" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Clear chat" }),
    ).toBeInTheDocument();
  });
});
