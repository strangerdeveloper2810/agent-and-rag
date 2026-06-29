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

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = `${Math.min(el.scrollHeight, 200)}px`;
  }, [value]);

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      onSend();
    }
  };

  const canSend = value.trim().length > 0 && !disabled;

  return (
    <div className="px-4 pb-4 pt-2 sm:px-6">
      <div className="mx-auto max-w-3xl">
        <div className="flex items-end gap-2 rounded-[28px] bg-subtle px-2 py-1.5 transition focus-within:bg-subtle2">
          <textarea
            ref={ref}
            rows={1}
            value={value}
            onChange={(e) => onChange(e.target.value)}
            onKeyDown={handleKeyDown}
            disabled={disabled}
            placeholder="Nhập câu lệnh tại đây"
            className="scroll-fine max-h-52 flex-1 resize-none bg-transparent px-4 py-2.5 text-[0.95rem] leading-relaxed text-ink outline-none placeholder:text-ink-faint disabled:opacity-60"
          />
          <button
            type="button"
            onClick={onSend}
            disabled={!canSend}
            aria-label="Gửi tin nhắn"
            className={`mb-0.5 flex h-10 w-10 shrink-0 items-center justify-center rounded-full transition ${
              canSend
                ? "bg-gblue text-white hover:bg-gblue-bright"
                : "cursor-not-allowed bg-line text-ink-faint"
            }`}
          >
            <SendIcon width={19} height={19} />
          </button>
        </div>
        <p className="mt-2 text-center text-xs text-ink-faint">
          Agent Tut có thể mắc lỗi. Hãy kiểm chứng thông tin quan trọng.
        </p>
      </div>
    </div>
  );
}
