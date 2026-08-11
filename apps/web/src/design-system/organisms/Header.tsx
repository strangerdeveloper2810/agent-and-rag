import ThemeToggle from "@/shared/components/ThemeToggle";

export interface HeaderProps {
  onToggleSidebar: () => void;
  title?: string;
}

/**
 * Header organism component providing top navigation bar with sidebar toggle and theme switcher.
 */
export const Header: React.FC<HeaderProps> = ({
  onToggleSidebar,
  title = "J.A.R.V.I.S.",
}) => {
  return (
    <header
      className="flex shrink-0 items-center justify-between border-b px-4 py-3 sm:px-6 transition-all duration-200 backdrop-blur-xl"
      style={{
        borderColor: "var(--border)",
        backgroundColor: "var(--surface)",
      }}
    >
      <div className="flex items-center gap-3">
        <button
          type="button"
          onClick={onToggleSidebar}
          aria-label="Toggle sidebar"
          className="flex h-8 w-8 items-center justify-center rounded-lg transition duration-150 hover:bg-[var(--bg-hover)] active:scale-95 border"
          style={{ color: "var(--text-secondary)", borderColor: "var(--border)" }}
        >
          <svg
            width={18}
            height={18}
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth={1.8}
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <path d="M4 6h16M4 12h16M4 18h16" />
          </svg>
        </button>

        <div className="flex items-center gap-2.5">
          <span className="font-display text-sm font-extrabold tracking-tight text-[var(--text)]">
            {title}
          </span>
          <span className="flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-mono text-[var(--success)] bg-[var(--success-bg)] border border-[var(--success)]/20">
            <span className="h-1.5 w-1.5 rounded-full bg-[var(--success)] animate-pulse" />
            ONLINE
          </span>
        </div>
      </div>

      <div className="flex items-center gap-3">
        <ThemeToggle />
      </div>
    </header>
  );
};

export default Header;
