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
import { CheckIcon, CloseIcon } from "@app/ui";

type ToastType = "success" | "error" | "info" | "warning";
type ToastItem = { id: number; type: ToastType; message: string };

type ToastApi = {
  success: (message: string) => void;
  error: (message: string) => void;
  info: (message: string) => void;
  warning: (message: string) => void;
};

const ToastContext = createContext<ToastApi | null>(null);

/**
 * Custom hook to access toast notification API.
 *
 * @returns ToastApi object with success and error methods
 */
export const useToast = (): ToastApi => {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error("useToast must be used inside <ToastProvider>");
  return ctx;
};

const DURATION = 3500;

/**
 * ToastProvider component managing global toast notification stack and context.
 */
export const ToastProvider: React.FC<{ children: ReactNode }> = ({
  children,
}) => {
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
      info: (message) => push("info", message),
      warning: (message) => push("warning", message),
    };
  }, []);

  return (
    <ToastContext.Provider value={api}>
      {children}
      <div className="pointer-events-none fixed top-4 right-4 z-[60] flex w-full max-w-sm flex-col gap-2">
        {toasts.map((t) => (
          <Toast key={t.id} toast={t} onClose={() => remove(t.id)} />
        ))}
      </div>
    </ToastContext.Provider>
  );
};

/**
 * Individual Toast item component displaying notification message with auto-dismiss.
 */
const Toast: React.FC<{ toast: ToastItem; onClose: () => void }> = ({
  toast,
  onClose,
}) => {
  useEffect(() => {
    const timer = setTimeout(onClose, DURATION);
    return () => clearTimeout(timer);
  }, [onClose]);

  const isError = toast.type === "error";
  const isSuccess = toast.type === "success";
  const isInfo = toast.type === "info";
  const isWarning = toast.type === "warning";

  const accentColor = isError
    ? "var(--danger)"
    : isWarning
      ? "var(--warning, #f59e0b)"
      : isInfo
        ? "var(--info, #3b82f6)"
        : "var(--success)";

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
        {isError && <CloseIcon width={13} height={13} />}
        {isSuccess && <CheckIcon width={13} height={13} />}
        {isInfo && <InfoSvg />}
        {isWarning && <WarnSvg />}
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
};

/** Icon "i" cho toast info. */
const InfoSvg: React.FC = () => (
  <svg
    width="13"
    height="13"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    strokeWidth="2.5"
    strokeLinecap="round"
    strokeLinejoin="round"
  >
    <circle cx="12" cy="12" r="10" />
    <line x1="12" y1="16" x2="12" y2="12" />
    <line x1="12" y1="8" x2="12.01" y2="8" />
  </svg>
);

/** Icon "!" cho toast warning. */
const WarnSvg: React.FC = () => (
  <svg
    width="13"
    height="13"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    strokeWidth="2.5"
    strokeLinecap="round"
    strokeLinejoin="round"
  >
    <path d="M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z" />
    <line x1="12" y1="9" x2="12" y2="13" />
    <line x1="12" y1="17" x2="12.01" y2="17" />
  </svg>
);
