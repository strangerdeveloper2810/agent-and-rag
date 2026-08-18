import { render, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "@/i18n";
import { SearchBar } from "./SearchBar";

const renderSearchBar = (value = "") =>
  render(
    <I18nextProvider i18n={i18n}>
      <SearchBar value={value} onChange={vi.fn()} />
    </I18nextProvider>,
  );

describe("SearchBar", () => {
  beforeEach(async () => {
    await i18n.changeLanguage("vi");
  });

  it("uses the translated default placeholder in Vietnamese", () => {
    renderSearchBar();
    expect(screen.getByPlaceholderText("Tìm kiếm...")).toBeInTheDocument();
  });

  it("uses the translated default placeholder in English", async () => {
    await i18n.changeLanguage("en");
    renderSearchBar();
    expect(screen.getByPlaceholderText("Search...")).toBeInTheDocument();
  });

  it("still allows overriding the placeholder via prop", () => {
    render(
      <I18nextProvider i18n={i18n}>
        <SearchBar value="" onChange={vi.fn()} placeholder="Custom text" />
      </I18nextProvider>,
    );
    expect(screen.getByPlaceholderText("Custom text")).toBeInTheDocument();
  });

  it("shows a translated clear button aria-label in Vietnamese when a value is present", () => {
    renderSearchBar("abc");
    expect(
      screen.getByRole("button", { name: "Xóa tìm kiếm" }),
    ).toBeInTheDocument();
  });

  it("shows a translated clear button aria-label in English when a value is present", async () => {
    await i18n.changeLanguage("en");
    renderSearchBar("abc");
    expect(
      screen.getByRole("button", { name: "Clear search" }),
    ).toBeInTheDocument();
  });
});
