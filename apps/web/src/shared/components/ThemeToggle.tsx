import { useEffect, useState, useCallback } from "react";
import { SunIcon, MoonIcon } from "./icons";

type Theme = "light" | "dark";

const STORAGE_KEY = "theme";
const DARK_CLASS = "dark";

function getSystemTheme(): Theme {
  if (typeof window === "undefined") return "light";
  return window.matchMedia("(prefers-color-scheme: dark)").matches
    ? "dark"
    : "light";
}

function getStoredTheme(): Theme | null {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored === "light" || stored === "dark") return stored;
  } catch {
    // localStorage unavailable
  }
  return null;
}

function applyTheme(theme: Theme) {
  const root = document.documentElement;
  if (theme === "dark") {
    root.classList.add(DARK_CLASS);
  } else {
    root.classList.remove(DARK_CLASS);
  }
}

let globalTheme: Theme | null = null;
const listeners = new Set<(t: Theme) => void>();

function notifyListeners(theme: Theme) {
  globalTheme = theme;
  for (const fn of listeners) fn(theme);
}

export function subscribeTheme(fn: (t: Theme) => void): () => void {
  listeners.add(fn);
  return () => listeners.delete(fn);
}

export function getCurrentTheme(): Theme {
  if (globalTheme) return globalTheme;
  return getStoredTheme() ?? getSystemTheme();
}

/** Initialize theme before React hydrates to prevent flash. Call once in main.tsx. */
export function initTheme() {
  const theme = getStoredTheme() ?? getSystemTheme();
  applyTheme(theme);
  globalTheme = theme;
}

export default function ThemeToggle() {
  const [theme, setTheme] = useState<Theme>(
    () => globalTheme ?? getStoredTheme() ?? getSystemTheme(),
  );

  useEffect(() => {
    const unsub = subscribeTheme(setTheme);
    return unsub;
  }, []);

  const toggle = useCallback(() => {
    setTheme((prev) => {
      const next = prev === "dark" ? "light" : "dark";
      applyTheme(next);
      try {
        localStorage.setItem(STORAGE_KEY, next);
      } catch {
        // ignore
      }
      notifyListeners(next);
      return next;
    });
  }, []);

  return (
    <button
      type="button"
      onClick={toggle}
      aria-label={`Switch to ${theme === "dark" ? "light" : "dark"} mode`}
      className="rounded-full p-2 text-ink-soft transition hover:bg-subtle hover:text-ink"
      title={theme === "dark" ? "Light mode" : "Dark mode"}
    >
      {theme === "dark" ? (
        <SunIcon width={18} height={18} />
      ) : (
        <MoonIcon width={18} height={18} />
      )}
    </button>
  );
}
