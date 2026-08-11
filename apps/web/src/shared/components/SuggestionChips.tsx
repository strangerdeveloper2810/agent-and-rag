export interface SuggestionChipsProps {
  suggestions: string[];
  onPick: (prompt: string) => void;
}

/**
 * SuggestionChips component for rendering a grid of interactive prompt recommendations.
 */
export const SuggestionChips: React.FC<SuggestionChipsProps> = ({
  suggestions,
  onPick,
}) => {
  if (suggestions.length === 0) return null;

  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2" role="list">
      {suggestions.map((s) => (
        <button
          key={s}
          type="button"
          onClick={() => onPick(s)}
          role="listitem"
          className="group relative flex items-center justify-between rounded-xl p-4 text-left text-xs font-medium leading-relaxed transition-all duration-200 hover:border-[var(--accent)] hover:bg-[var(--bg-hover)]"
          style={{
            backgroundColor: "var(--surface)",
            border: "1px solid var(--border)",
            color: "var(--text)",
          }}
        >
          <span className="min-w-0 flex-1 truncate pr-2 group-hover:text-[var(--accent)] transition-colors">
            {s}
          </span>
          <svg
            width={14}
            height={14}
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth={2}
            strokeLinecap="round"
            strokeLinejoin="round"
            className="shrink-0 opacity-40 transition group-hover:translate-x-0.5 group-hover:opacity-100 text-[var(--accent)]"
          >
            <path d="M5 12h14M12 5l7 7-7 7" />
          </svg>
        </button>
      ))}
    </div>
  );
};

export default SuggestionChips;
