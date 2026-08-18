import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";
import i18n from "@/i18n";
import { ModeSelector } from "./ModeSelector";

const renderSelector = () =>
  render(
    <I18nextProvider i18n={i18n}>
      <ModeSelector currentMode="auto" onSelectMode={() => {}} />
    </I18nextProvider>,
  );

describe("ModeSelector", () => {
  beforeEach(async () => {
    await i18n.changeLanguage("vi");
  });

  it("shows Vietnamese menu copy when opened", async () => {
    renderSelector();
    expect(
      screen.getByRole("button", { name: "Chọn chế độ AI" }),
    ).toBeInTheDocument();

    await userEvent.click(
      screen.getByRole("button", { name: "Chọn chế độ AI" }),
    );

    expect(screen.getByText("CHẾ ĐỘ AI INTELLIGENCE")).toBeInTheDocument();
    expect(
      screen.getByText("Tự động điều phối Agent phù hợp nhất theo câu hỏi"),
    ).toBeInTheDocument();
    expect(screen.getByText("Khuyên dùng")).toBeInTheDocument();
  });

  it("shows English menu copy when locale is en", async () => {
    await i18n.changeLanguage("en");
    renderSelector();

    expect(
      screen.getByRole("button", { name: "Select AI mode" }),
    ).toBeInTheDocument();

    await userEvent.click(
      screen.getByRole("button", { name: "Select AI mode" }),
    );

    expect(screen.getByText("AI INTELLIGENCE MODE")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Automatically routes to the most suitable agent for your question",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Recommended")).toBeInTheDocument();
  });
});
