import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";
import i18n from "@/i18n";
import { LANGUAGE_STORAGE_KEY } from "@/i18n/locale";
import { LanguageSwitcher } from "./LanguageSwitcher";

const renderSwitcher = () =>
  render(
    <I18nextProvider i18n={i18n}>
      <LanguageSwitcher />
    </I18nextProvider>,
  );

describe("LanguageSwitcher", () => {
  beforeEach(async () => {
    localStorage.clear();
    await i18n.changeLanguage("vi");
  });

  it("renders segmented VI and EN buttons and highlights active language", async () => {
    renderSwitcher();

    const viBtn = screen.getByRole("button", { name: "Tiếng Việt" });
    const enBtn = screen.getByRole("button", { name: "English" });

    expect(viBtn).toHaveAttribute("aria-pressed", "true");
    expect(enBtn).toHaveAttribute("aria-pressed", "false");

    await userEvent.click(enBtn);

    expect(i18n.language).toBe("en");
    expect(localStorage.getItem(LANGUAGE_STORAGE_KEY)).toBe("en");
    expect(enBtn).toHaveAttribute("aria-pressed", "true");
  });

  it("switches back to Vietnamese when already in English", async () => {
    await i18n.changeLanguage("en");
    renderSwitcher();

    const viBtn = screen.getByRole("button", { name: "Tiếng Việt" });
    const enBtn = screen.getByRole("button", { name: "English" });

    expect(enBtn).toHaveAttribute("aria-pressed", "true");

    await userEvent.click(viBtn);

    expect(i18n.language).toBe("vi");
    expect(localStorage.getItem(LANGUAGE_STORAGE_KEY)).toBe("vi");
    expect(viBtn).toHaveAttribute("aria-pressed", "true");
  });
});
