import { render, screen } from "@testing-library/react";
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

  it("shows the Vietnamese label for the target theme", () => {
    renderToggle();
    expect(screen.getByRole("button")).toHaveTextContent("Sáng");
  });

  it("shows the English label when locale is en", async () => {
    await i18n.changeLanguage("en");
    renderToggle();
    expect(screen.getByRole("button")).toHaveTextContent("Light");
  });
});
