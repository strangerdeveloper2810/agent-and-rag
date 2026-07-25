import React from "react";

export interface NavTabProps {
  active: boolean;
  onClick: () => void;
  icon?: React.ReactNode;
  children: React.ReactNode;
}

/**
 * NavTab molecule component for sidebar view switching (Chat / Documents).
 */
export const NavTab: React.FC<NavTabProps> = ({ active, onClick, icon, children }) => {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`flex items-center justify-center gap-2 rounded-xl py-2 text-[12px] font-medium transition-all duration-150 active:scale-[0.98] ${
        active
          ? "bg-[var(--accent-bg)] font-semibold shadow-sm border border-[var(--accent)]/30"
          : "hover:bg-[var(--bg-raised)]"
      }`}
      style={{
        color: active ? "var(--accent)" : "var(--text-secondary)",
      }}
    >
      {icon}
      {children}
    </button>
  );
};

export default NavTab;
