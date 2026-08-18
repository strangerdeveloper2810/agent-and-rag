import { useTranslation } from "react-i18next";
import { LinkIcon } from "@app/ui";
import type { CitationData } from "@/modules/chat/chat.api";

export interface CitationListProps {
  citations: CitationData[];
}

/**
 * CitationList component for rendering document and web citations referenced by the agent.
 */
export const CitationList: React.FC<CitationListProps> = ({ citations }) => {
  const { t } = useTranslation("chat");

  if (citations.length === 0) return null;

  return (
    <div
      className="mt-3 border-t pt-3"
      style={{ borderColor: "var(--border)" }}
      aria-label="Sources"
    >
      <p
        className="mb-2 text-[10px] font-medium uppercase tracking-wider"
        style={{ color: "var(--text-tertiary)" }}
      >
        {citations.length === 1
          ? t("citationList.source")
          : t("citationList.sources")}
      </p>
      <ul className="flex flex-wrap gap-1.5">
        {citations.map((c, i) => (
          <li key={i}>
            {c.url ? (
              <a
                href={c.url}
                target="_blank"
                rel="noopener noreferrer"
                className="neon-tag"
                title={c.snippet}
              >
                <LinkIcon width={12} height={12} />
                <span className="truncate max-w-[160px]">{c.title}</span>
              </a>
            ) : (
              <span
                className="inline-flex items-center gap-1 rounded-full border px-2.5 py-1 text-[11px]"
                style={{
                  borderColor: "var(--border)",
                  color: "var(--text-secondary)",
                  backgroundColor: "var(--bg-raised)",
                }}
              >
                {c.title}
              </span>
            )}
          </li>
        ))}
      </ul>
    </div>
  );
};

export default CitationList;
