import { SparkIcon } from "./icons";

const SUGGESTIONS = [
  "Giải thích RAG hoạt động thế nào",
  "Viết một đoạn code TypeScript debounce",
  "Tóm tắt ưu nhược điểm của MongoDB",
  "Gợi ý cách học AI Agent từ cơ bản",
];

export default function EmptyState({
  onPick,
}: {
  onPick: (prompt: string) => void;
}) {
  return (
    <div className="flex h-full flex-col items-center justify-center px-6 text-center animate-fade-in">
      <div className="mb-5 flex h-14 w-14 items-center justify-center rounded-2xl bg-gradient-to-br from-accent-glow to-accent-ink text-white shadow-bubble">
        <SparkIcon width={26} height={26} />
      </div>

      <h1 className="font-display text-3xl font-bold tracking-tight text-ink sm:text-4xl">
        Xin chào 👋
      </h1>
      <p className="mt-2 max-w-md text-ink-soft">
        Mình là <span className="font-medium text-accent-ink">Agent Tut</span>.
        Hỏi mình bất cứ điều gì, hoặc thử một gợi ý bên dưới để bắt đầu.
      </p>

      <div className="mt-8 grid w-full max-w-lg grid-cols-1 gap-2.5 sm:grid-cols-2">
        {SUGGESTIONS.map((s) => (
          <button
            key={s}
            type="button"
            onClick={() => onPick(s)}
            className="rounded-2xl border border-line bg-surface/70 px-4 py-3 text-left text-sm text-ink-soft shadow-ring transition hover:-translate-y-0.5 hover:border-accent/40 hover:text-ink hover:shadow-soft"
          >
            {s}
          </button>
        ))}
      </div>
    </div>
  );
}
