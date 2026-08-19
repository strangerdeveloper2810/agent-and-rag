import React, { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { SunIcon, MoonIcon } from "@heroicons/react/24/outline";

const KEY = "jarvis-theme";

/**
 * Initializes HTML root data-theme attribute & dark class on app load.
 */
export const initTheme = (): void => {
  const saved = localStorage.getItem(KEY) || "dark";
  document.documentElement.setAttribute("data-theme", saved);
  if (saved === "dark") {
    document.documentElement.classList.add("dark");
  } else {
    document.documentElement.classList.remove("dark");
  }
};

/**
 * ThemeToggle component for switching between Dark & Light modes.
 */
export const ThemeToggle: React.FC = () => {
  const { t } = useTranslation();
  const [theme, setTheme] = useState<"dark" | "light">(() => {
    if (typeof localStorage === "undefined") return "dark"; // SSR (landing prerender)
    return (localStorage.getItem(KEY) as "dark" | "light") || "dark";
  });

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", theme);
    if (theme === "dark") {
      document.documentElement.classList.add("dark");
    } else {
      document.documentElement.classList.remove("dark");
    }
    localStorage.setItem(KEY, theme);
  }, [theme]);

  const toggle = () => setTheme((t) => (t === "dark" ? "light" : "dark"));
  const isDark = theme === "dark";

  return (
    <button
      type="button"
      onClick={toggle}
      aria-label={t("theme.switchTo", {
        mode: isDark ? t("theme.light") : t("theme.dark"),
      })}
      title={isDark ? "Chuyển sang Chế độ Sáng" : "Chuyển sang Chế độ Tối"}
      className="inline-flex items-center justify-center h-8 w-8 rounded-full border border-border/50 bg-muted/60 text-muted-foreground hover:text-foreground hover:bg-muted hover:border-border transition-all duration-200 shadow-xs"
    >
      {isDark ? (
        <SunIcon className="h-4 w-4 text-amber-400 transition-transform duration-300 hover:rotate-45" />
      ) : (
        <MoonIcon className="h-4 w-4 text-indigo-600 transition-transform duration-300 hover:-rotate-12" />
      )}
    </button>
  );
};

export default ThemeToggle;
