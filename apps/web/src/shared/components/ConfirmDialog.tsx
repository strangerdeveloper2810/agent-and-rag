import { useEffect } from "react";

export default function ConfirmDialog({
  open,
  title,
  message,
  confirmLabel = "Confirm",
  cancelLabel = "Cancel",
  danger = false,
  onConfirm,
  onCancel,
}: {
  open: boolean;
  title: string;
  message?: string;
  confirmLabel?: string;
  cancelLabel?: string;
  danger?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onCancel();
      if (e.key === "Enter") onConfirm();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open, onCancel, onConfirm]);

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 animate-fade-in"
      onClick={onCancel}
      role="dialog"
      aria-modal="true"
    >
      <div
        className="w-full max-w-sm rounded-2xl p-6 shadow-2xl"
        style={{
          backgroundColor: "var(--cyber-surface)",
          border: "1px solid var(--cyber-border)",
          boxShadow: "0 0 30px rgba(0,0,0,0.5), 0 0 10px rgba(0,240,255,0.05)",
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <h2 className="text-base font-medium" style={{ color: "var(--cyber-text)" }}>
          {title}
        </h2>
        {message && (
          <p className="mt-2 text-xs leading-relaxed" style={{ color: "var(--cyber-muted)" }}>
            {message}
          </p>
        )}
        <div className="mt-6 flex justify-end gap-2">
          <button
            type="button"
            onClick={onCancel}
            className="rounded-lg px-4 py-2 text-xs font-medium transition hover:bg-[var(--cyber-subtle)]"
            style={{ color: "var(--cyber-muted)" }}
          >
            {cancelLabel}
          </button>
          <button
            type="button"
            onClick={onConfirm}
            className="rounded-lg px-4 py-2 text-xs font-medium transition"
            style={{
              backgroundColor: danger ? "var(--cyber-error)" : "var(--cyber-primary)",
              color: "#0a0a0f",
            }}
          >
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
}
