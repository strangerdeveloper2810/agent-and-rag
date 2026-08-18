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

  it("shows the target language label and switches to English on click", async () => {
    renderSwitcher();

    const button = screen.getByRole("button");
    expect(button).toHaveTextContent("EN");

    await userEvent.click(button);

    expect(i18n.language).toBe("en");
    expect(localStorage.getItem(LANGUAGE_STORAGE_KEY)).toBe("en");
  });

  it("switches back to Vietnamese when already in English", async () => {
    await i18n.changeLanguage("en");
    renderSwitcher();

    const button = screen.getByRole("button");
    expect(button).toHaveTextContent("VI");

    await userEvent.click(button);

    expect(i18n.language).toBe("vi");
  });
});
