import { useEffect, useState } from "react";

const KEY = "jarvis-theme";

export function initTheme() {
  const saved = localStorage.getItem(KEY) || "dark";
  document.documentElement.setAttribute("data-theme", saved);
}

export default function ThemeToggle() {
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
}

function SunIcon() {
  return (
    <svg
      width={15}
      height={15}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.6}
    >
      <circle cx={12} cy={12} r={5} />
      <path d="M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42" />
    </svg>
  );
}

function MoonIcon() {
  return (
    <svg
      width={15}
      height={15}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.6}
    >
      <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
    </svg>
  );
}
