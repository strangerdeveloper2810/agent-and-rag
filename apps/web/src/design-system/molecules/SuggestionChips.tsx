import React from "react";
import { SparklesIcon } from "@heroicons/react/24/outline";

export interface SuggestionChipsProps {
  suggestions: string[];
  disabled?: boolean;
  onSelect: (promptText: string) => void;
}

export const SuggestionChips: React.FC<SuggestionChipsProps> = ({
  suggestions,
  disabled = false,
  onSelect,
}) => {
  if (!suggestions || suggestions.length === 0) return null;

  return (
    <div className="mt-3.5 pt-2 flex flex-col gap-1.5">
      <div className="flex items-center gap-1.5 text-[11px] font-medium text-muted-foreground">
        <SparklesIcon className="h-3.5 w-3.5 text-primary" />
        <span>Gợi ý bước tiếp theo:</span>
      </div>
      <div className="flex flex-wrap gap-2">
        {suggestions.map((s, idx) => (
          <button
            key={`${idx}-${s}`}
            type="button"
            disabled={disabled}
            onClick={() => onSelect(s)}
            className="group inline-flex items-center gap-1.5 rounded-full border border-primary/20 bg-primary/5 px-3 py-1.5 text-xs text-foreground transition-all duration-200 hover:border-primary hover:bg-primary/10 hover:shadow-sm active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed text-left cursor-pointer"
          >
            <span className="group-hover:text-primary transition-colors">{s}</span>
            <span className="text-primary font-bold opacity-60 group-hover:opacity-100 group-hover:translate-x-0.5 transition-all">
              →
            </span>
          </button>
        ))}
      </div>
    </div>
  );
};

export default SuggestionChips;
