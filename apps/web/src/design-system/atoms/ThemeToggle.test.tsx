import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";
import i18n from "@/i18n";
import { ThemeToggle } from "./ThemeToggle";

const renderToggle = () =>
  render(
    <I18nextProvider i18n={i18n}>
      <ThemeToggle />
    </I18nextProvider>,
  );

describe("ThemeToggle", () => {
  beforeEach(async () => {
    localStorage.clear();
    localStorage.setItem("jarvis-theme", "dark");
    await i18n.changeLanguage("vi");
  });

  it("shows the Vietnamese aria-label for switching theme and toggles theme on click", async () => {
    renderToggle();

    const button = screen.getByRole("button");
    expect(button).toHaveAttribute("aria-label", "Chuyển sang chế độ Sáng");

    await userEvent.click(button);

    expect(localStorage.getItem("jarvis-theme")).toBe("light");
    expect(button).toHaveAttribute("aria-label", "Chuyển sang chế độ Tối");
  });

  it("shows the English aria-label when locale is en", async () => {
    await i18n.changeLanguage("en");
    renderToggle();

    const button = screen.getByRole("button");
    expect(button).toHaveAttribute("aria-label", "Switch to Light mode");
  });
});
