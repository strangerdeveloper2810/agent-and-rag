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

/** Gọi trong component để bắn thông báo: toast.success(...) / toast.error(...). */
export function useToast(): ToastApi {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error("useToast phải dùng bên trong <ToastProvider>");
  return ctx;
}

const DURATION = 3500;

/** Bọc app để cung cấp toast toàn cục + render ngăn xếp toast (góc dưới phải). */
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

  return (
    <div
      role="status"
      className="pointer-events-auto flex items-start gap-3 rounded-2xl bg-surface p-3 pr-2 shadow-soft animate-msg-in"
    >
      <span
        className={`mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-white ${
          isError ? "bg-red-600" : "bg-green-600"
        }`}
      >
        {isError ? (
          <CloseIcon width={14} height={14} />
        ) : (
          <CheckIcon width={14} height={14} />
        )}
      </span>
      <p className="min-w-0 flex-1 py-0.5 text-sm text-ink-soft">
        {toast.message}
      </p>
      <button
        type="button"
        onClick={onClose}
        aria-label="Đóng thông báo"
        className="rounded-full p-1.5 text-ink-faint transition hover:bg-subtle"
      >
        <CloseIcon width={14} height={14} />
      </button>
    </div>
  );
}
