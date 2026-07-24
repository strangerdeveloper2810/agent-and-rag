/**
 * Replaces the old light/dark toggle.
 * Shows "SYS ONLINE" indicator — cyberpunk dark-only.
 * Keeps initTheme stub for backward compat.
 */

/** No-op: always dark. */
export function initTheme() {}

export default function SysOnline() {
  return (
    <div className="flex items-center gap-2" aria-label="System online">
      <span className="relative flex h-2 w-2">
        <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-[var(--cyber-success)] opacity-75" />
        <span className="relative inline-flex h-2 w-2 rounded-full bg-[var(--cyber-success)]" />
      </span>
      <span className="text-[10px] font-medium tracking-widest text-[var(--cyber-faint)] uppercase">
        SYS ONLINE
      </span>
    </div>
  );
}
