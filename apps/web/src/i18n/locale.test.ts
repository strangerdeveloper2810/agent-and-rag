import { afterEach, beforeEach, describe, expect, it } from "vitest";
import {
  LANGUAGE_STORAGE_KEY,
  getInitialLocale,
  persistLocale,
} from "./locale";

describe("getInitialLocale", () => {
  const originalLanguage = window.navigator.language;

  const setNavigatorLanguage = (lang: string) => {
    Object.defineProperty(window.navigator, "language", {
      value: lang,
      configurable: true,
    });
  };

  beforeEach(() => {
    localStorage.clear();
  });

  afterEach(() => {
    setNavigatorLanguage(originalLanguage);
  });

  it("returns the locale saved in localStorage when present", () => {
    localStorage.setItem(LANGUAGE_STORAGE_KEY, "en");
    setNavigatorLanguage("vi-VN");

    expect(getInitialLocale()).toBe("en");
  });

  it("ignores an invalid value saved in localStorage", () => {
    localStorage.setItem(LANGUAGE_STORAGE_KEY, "fr");
    setNavigatorLanguage("en-US");

    expect(getInitialLocale()).toBe("en");
  });

  it("falls back to the browser language when nothing is saved", () => {
    setNavigatorLanguage("en-US");

    expect(getInitialLocale()).toBe("en");
  });

  it("defaults to vi when browser language is neither vi nor en", () => {
    setNavigatorLanguage("fr-FR");

    expect(getInitialLocale()).toBe("vi");
  });
});

describe("persistLocale", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it("saves the locale to localStorage", () => {
    persistLocale("en");

    expect(localStorage.getItem(LANGUAGE_STORAGE_KEY)).toBe("en");
  });
});
