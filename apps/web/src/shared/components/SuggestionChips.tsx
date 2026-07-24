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
          className="rounded-xl px-4 py-3.5 text-left text-xs leading-relaxed transition-all duration-200 hover:border-[var(--cyber-primary)] hover:shadow-[0_0_12px_rgba(0,240,255,0.15)] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--cyber-primary)]"
          style={{
            backgroundColor: "var(--cyber-subtle)",
            border: "1px solid var(--cyber-border)",
            color: "var(--cyber-muted)",
          }}
        >
          <span className="mr-1.5 opacity-50">&gt;</span>
          {s}
        </button>
      ))}
    </div>
  );
}
