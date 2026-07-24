import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { CheckIcon, CloseIcon } from "./icons";

type ToastType = "success" | "error";
type ToastItem = { id: number; type: ToastType; message: string };

type ToastApi = {
  success: (message: string) => void;
  error: (message: string) => void;
};

const ToastContext = createContext<ToastApi | null>(null);

export function useToast(): ToastApi {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error("useToast must be used inside <ToastProvider>");
  return ctx;
}

const DURATION = 3500;

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<ToastItem[]>([]);
  const idRef = useRef(0);

  const remove = useCallback((id: number) => {
    setToasts((list) => list.filter((t) => t.id !== id));
  }, []);

  const api = useMemo<ToastApi>(() => {
    const push = (type: ToastType, message: string) => {
      const id = ++idRef.current;
      setToasts((list) => [...list, { id, type, message }]);
    };
    return {
      success: (message) => push("success", message),
      error: (message) => push("error", message),
    };
  }, []);

  return (
    <ToastContext.Provider value={api}>
      {children}
      <div className="pointer-events-none fixed bottom-4 right-4 z-[60] flex w-full max-w-sm flex-col gap-2">
        {toasts.map((t) => (
          <Toast key={t.id} toast={t} onClose={() => remove(t.id)} />
        ))}
      </div>
    </ToastContext.Provider>
  );
}

function Toast({ toast, onClose }: { toast: ToastItem; onClose: () => void }) {
  useEffect(() => {
    const timer = setTimeout(onClose, DURATION);
    return () => clearTimeout(timer);
  }, [onClose]);

  const isError = toast.type === "error";
  const accentColor = isError ? "var(--danger)" : "var(--success)";

  return (
    <div
      role="status"
      className="pointer-events-auto flex items-start gap-3 rounded-xl p-3 pr-2 animate-msg-in"
      style={{
        backgroundColor: "var(--surface)",
        border: "1px solid var(--border)",
        boxShadow: `0 0 10px rgba(0,0,0,0.5), 0 0 4px ${accentColor}20`,
      }}
    >
      <span
        className="mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-full"
        style={{
          backgroundColor: accentColor,
          color: "#0a0a0f",
        }}
      >
        {isError ? (
          <CloseIcon width={13} height={13} />
        ) : (
          <CheckIcon width={13} height={13} />
        )}
      </span>
      <p
        className="min-w-0 flex-1 py-0.5 text-xs"
        style={{ color: "var(--text)" }}
      >
        {toast.message}
      </p>
      <button
        type="button"
        onClick={onClose}
        aria-label="Dismiss"
        className="rounded-full p-1.5 transition hover:bg-[var(--bg-raised)]"
        style={{ color: "var(--text-tertiary)" }}
      >
        <CloseIcon width={13} height={13} />
      </button>
    </div>
  );
}
