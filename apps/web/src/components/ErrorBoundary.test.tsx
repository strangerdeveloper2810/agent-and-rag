import { render, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "@/i18n";
import { ErrorBoundary } from "./ErrorBoundary";

const Boom: React.FC = () => {
  throw new Error("kaboom");
};

const renderBoundary = () =>
  render(
    <I18nextProvider i18n={i18n}>
      <ErrorBoundary>
        <Boom />
      </ErrorBoundary>
    </I18nextProvider>,
  );

describe("ErrorBoundary", () => {
  beforeEach(async () => {
    await i18n.changeLanguage("vi");
    vi.spyOn(console, "error").mockImplementation(() => {});
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("shows the Vietnamese fallback UI", () => {
    renderBoundary();
    expect(screen.getByText("Đã xảy ra lỗi")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Thử lại" })).toBeInTheDocument();
  });

  it("shows the English fallback UI", async () => {
    await i18n.changeLanguage("en");
    renderBoundary();
    expect(screen.getByText("Something went wrong")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
  });
});
