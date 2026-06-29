const SUGGESTIONS = [
  "Giải thích RAG hoạt động thế nào",
  "Tạo task họp nhóm thứ 5 ưu tiên cao",
  "Trong tài liệu có thông tin gì?",
  "Có bao nhiêu tài liệu đã nạp?",
];

export default function EmptyState({
  onPick,
}: {
  onPick: (prompt: string) => void;
}) {
  return (
    <div className="mx-auto flex h-full max-w-3xl flex-col justify-center px-4 sm:px-6 animate-fade-in">
      <h1 className="text-4xl font-medium tracking-tight sm:text-5xl">
        <span className="text-gemini">Xin chào</span>
      </h1>
      <p className="mt-2 text-2xl font-medium text-ink-faint sm:text-3xl">
        Hôm nay tôi giúp được gì cho bạn?
      </p>

      <div className="mt-10 grid grid-cols-1 gap-3 sm:grid-cols-2">
        {SUGGESTIONS.map((s) => (
          <button
            key={s}
            type="button"
            onClick={() => onPick(s)}
            className="rounded-2xl bg-subtle px-4 py-4 text-left text-sm text-ink-soft transition hover:bg-subtle2 hover:shadow-soft"
          >
            {s}
          </button>
        ))}
      </div>
    </div>
  );
}
