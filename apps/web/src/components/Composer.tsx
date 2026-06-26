import { useEffect, useRef } from "react";
import { SendIcon } from "./icons";

export default function Composer({
  value,
  onChange,
  onSend,
  disabled,
}: {
  value: string;
  onChange: (v: string) => void;
  onSend: () => void;
  disabled: boolean;
}) {
  const ref = useRef<HTMLTextAreaElement>(null);

  // Tự co giãn chiều cao theo nội dung (tối đa ~160px rồi cuộn)
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = `${Math.min(el.scrollHeight, 160)}px`;
  }, [value]);

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    // Enter để gửi, Shift+Enter để xuống dòng
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      onSend();
    }
  };

  const canSend = value.trim().length > 0 && !disabled;

  return (
    <div className="px-4 pb-4 pt-2 sm:px-6">
      <div className="mx-auto max-w-3xl">
        <div className="flex items-end gap-2 rounded-3xl border border-line bg-surface p-2 shadow-soft transition focus-within:border-accent/50 focus-within:shadow-bubble">
          <textarea
            ref={ref}
            rows={1}
            value={value}
            onChange={(e) => onChange(e.target.value)}
            onKeyDown={handleKeyDown}
            disabled={disabled}
            placeholder="Hỏi Agent điều gì đó…"
            className="scroll-fine max-h-40 flex-1 resize-none bg-transparent px-3 py-2 text-[0.95rem] leading-relaxed text-ink outline-none placeholder:text-ink-faint disabled:opacity-60"
          />
          <button
            type="button"
            onClick={onSend}
            disabled={!canSend}
            aria-label="Gửi tin nhắn"
            className="flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl bg-accent text-white shadow-bubble transition hover:bg-accent-ink disabled:cursor-not-allowed disabled:bg-ink-faint disabled:shadow-none"
          >
            <SendIcon width={19} height={19} />
          </button>
        </div>
        <p className="mt-2 text-center text-xs text-ink-faint">
          <kbd className="font-sans">Enter</kbd> để gửi ·{" "}
          <kbd className="font-sans">Shift</kbd> +{" "}
          <kbd className="font-sans">Enter</kbd> xuống dòng
        </p>
      </div>
    </div>
  );
}
