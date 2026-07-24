interface SuggestionChipsProps {
  suggestions: string[];
  onPick: (prompt: string) => void;
}

export default function SuggestionChips({
  suggestions,
  onPick,
}: SuggestionChipsProps) {
  if (suggestions.length === 0) return null;

  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2" role="list">
      {suggestions.map((s) => (
        <button
          key={s}
          type="button"
          onClick={() => onPick(s)}
          role="listitem"
          className="rounded-2xl bg-subtle px-4 py-4 text-left text-sm text-ink-soft transition hover:bg-subtle2 hover:shadow-soft focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-gblue"
        >
          {s}
        </button>
      ))}
    </div>
  );
}
