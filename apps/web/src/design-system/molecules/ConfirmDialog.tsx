import { useEffect } from "react";
import { Button } from "@/components/ui/button";

export interface ConfirmDialogProps {
  open: boolean;
  title: string;
  message?: string;
  confirmLabel?: string;
  cancelLabel?: string;
  danger?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}

/**
 * ConfirmDialog component rendering a modal confirmation dialog for destructive or important actions.
 */
export const ConfirmDialog: React.FC<ConfirmDialogProps> = ({
  open,
  title,
  message,
  confirmLabel = "Confirm",
  cancelLabel = "Cancel",
  danger = false,
  onConfirm,
  onCancel,
}) => {
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
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4 animate-fade-in backdrop-blur-sm"
      onClick={onCancel}
      role="dialog"
      aria-modal="true"
    >
      <div
        className="w-full max-w-sm rounded-2xl p-6 shadow-2xl bg-card border border-border text-card-foreground"
        onClick={(e) => e.stopPropagation()}
      >
        <h2 className="text-base font-bold text-foreground">{title}</h2>
        {message && (
          <p className="mt-2 text-xs leading-relaxed text-muted-foreground">
            {message}
          </p>
        )}
        <div className="mt-6 flex justify-end gap-2.5">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={onCancel}
            className="text-xs"
          >
            {cancelLabel}
          </Button>
          <Button
            type="button"
            variant={danger ? "destructive" : "default"}
            size="sm"
            onClick={onConfirm}
            className="text-xs font-bold shadow-md"
          >
            {confirmLabel}
          </Button>
        </div>
      </div>
    </div>
  );
};

export default ConfirmDialog;
