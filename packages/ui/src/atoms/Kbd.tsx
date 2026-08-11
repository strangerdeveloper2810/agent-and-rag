import React from "react";

export function Kbd({ children }: { children: React.ReactNode }) {
  return (
    <kbd className="inline-block rounded px-1.5 py-0.5 text-[10px] font-mono text-[var(--text-tertiary)] border border-[var(--border)] bg-[var(--bg-raised)] shadow-2xs">
      {children}
    </kbd>
  );
}
