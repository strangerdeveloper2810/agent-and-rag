import { useTranslation } from "react-i18next";
import { SearchIcon, CloseIcon } from "@app/ui";

export interface SearchBarProps {
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  className?: string;
}

/**
 * SearchBar molecule component for filtering lists and conversations.
 */
export const SearchBar: React.FC<SearchBarProps> = ({
  value,
  onChange,
  placeholder,
  className = "",
}) => {
  const { t } = useTranslation("layout");
  const resolvedPlaceholder = placeholder ?? t("searchBar.placeholder");

  return (
    <div
      className={`flex items-center gap-2 rounded-xl border px-3 py-1.5 transition-all focus-within:border-[var(--accent)] focus-within:ring-1 focus-within:ring-[var(--accent-bg)] ${className}`}
      style={{
        borderColor: "var(--border)",
        backgroundColor: "var(--bg-raised)",
      }}
    >
      <SearchIcon
        width={13}
        height={13}
        style={{ color: "var(--text-tertiary)" }}
        className="shrink-0"
      />
      <input
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={resolvedPlaceholder}
        aria-label={resolvedPlaceholder}
        className="flex-1 bg-transparent text-[11px] outline-none placeholder:text-[var(--text-tertiary)]"
        style={{ color: "var(--text)" }}
      />
      {value && (
        <button
          onClick={() => onChange("")}
          aria-label={t("searchBar.clearSearch")}
          className="rounded p-0.5 hover:text-[var(--text)] transition"
          style={{ color: "var(--text-tertiary)" }}
        >
          <CloseIcon width={11} height={11} />
        </button>
      )}
    </div>
  );
};

export default SearchBar;
