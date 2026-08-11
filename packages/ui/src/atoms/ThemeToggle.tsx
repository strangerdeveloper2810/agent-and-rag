import { useEffect, useState } from "react";
import { SunIcon, MoonIcon } from "../icons/icons";

const KEY = "jarvis-theme";

/**
 * Initializes HTML root data-theme attribute on app load.
 */
export const initTheme = (): void => {
  const saved = localStorage.getItem(KEY) || "dark";
  document.documentElement.setAttribute("data-theme", saved);
};

/**
 * ThemeToggle component for switching between Dark Cyber Console & Light modes.
 */
export const ThemeToggle: React.FC = () => {
  const [theme, setTheme] = useState<"dark" | "light">(() => {
    return (localStorage.getItem(KEY) as "dark" | "light") || "dark";
  });

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", theme);
    localStorage.setItem(KEY, theme);
  }, [theme]);

  const toggle = () => setTheme((t) => (t === "dark" ? "light" : "dark"));

  return (
    <button
      onClick={toggle}
      aria-label={`Switch to ${theme === "dark" ? "light" : "dark"} mode`}
      className="flex items-center gap-2 rounded-full px-3 py-1.5 text-xs transition hover:bg-[var(--border)]"
      style={{ color: "var(--text-secondary)" }}
    >
      {theme === "dark" ? <SunIcon /> : <MoonIcon />}
      <span className="hidden sm:inline tracking-wider uppercase text-[10px]">
        {theme === "dark" ? "Light" : "Dark"}
      </span>
    </button>
  );
};

export default ThemeToggle;
