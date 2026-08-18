import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { SunIcon, MoonIcon } from "@app/ui";

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
  const targetTheme = theme === "dark" ? "light" : "dark";

  return (
    <button
      onClick={toggle}
      aria-label={t("theme.switchTo", { mode: t(`theme.${targetTheme}`) })}
      className="flex items-center gap-2 rounded-full px-3 py-1.5 text-xs transition hover:bg-muted text-muted-foreground hover:text-foreground"
    >
      {theme === "dark" ? <SunIcon /> : <MoonIcon />}
      <span className="hidden sm:inline tracking-wider uppercase text-[10px]">
        {t(`theme.${targetTheme}`)}
      </span>
    </button>
  );
};

export default ThemeToggle;
