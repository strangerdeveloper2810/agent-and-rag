import { render, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";
import i18n from "@/i18n";
import { SlashCommandMenu } from "./SlashCommandMenu";

const renderMenu = () =>
  render(
    <I18nextProvider i18n={i18n}>
      <SlashCommandMenu filterText="" onSelect={() => {}} onClose={() => {}} />
    </I18nextProvider>,
  );

describe("SlashCommandMenu", () => {
  beforeEach(async () => {
    await i18n.changeLanguage("vi");
  });

  it("shows Vietnamese command labels", () => {
    renderMenu();
    expect(
      screen.getByText("SMART COMMANDS (PHÍM TẮT `/`)"),
    ).toBeInTheDocument();
    expect(screen.getByText("↑↓ chọn · Enter dùng")).toBeInTheDocument();
    expect(screen.getByText("Tóm tắt nhanh")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Tóm tắt ngắn gọn các điểm chính của tài liệu hoặc văn bản",
      ),
    ).toBeInTheDocument();
  });

  it("shows English command labels", async () => {
    await i18n.changeLanguage("en");
    renderMenu();
    expect(
      screen.getByText("SMART COMMANDS (`/` SHORTCUT)"),
    ).toBeInTheDocument();
    expect(screen.getByText("↑↓ to select · Enter to use")).toBeInTheDocument();
    expect(screen.getByText("Quick Summary")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Briefly summarize the key points of a document or text",
      ),
    ).toBeInTheDocument();
  });
});
