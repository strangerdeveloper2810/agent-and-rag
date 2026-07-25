import React from "react";

/**
 * Kbd atom component for rendering keyboard shortcuts.
 */
export const Kbd: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  return (
    <kbd className="inline-block rounded px-1.5 py-0.5 text-[10px] font-mono text-[var(--text-tertiary)] border border-[var(--border)] bg-[var(--bg-raised)] shadow-2xs">
      {children}
    </kbd>
  );
};

export default Kbd;
