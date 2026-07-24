import { LinkIcon } from "./icons";
import type { CitationData } from "@/modules/chat/chat.api";

interface CitationListProps {
  citations: CitationData[];
}

export default function CitationList({ citations }: CitationListProps) {
  if (citations.length === 0) return null;

  return (
    <div className="mt-3 border-t border-line pt-3" aria-label="Sources">
      <p className="mb-2 text-xs font-medium text-ink-faint">
        {citations.length === 1 ? "Source" : "Sources"}
      </p>
      <ul className="space-y-1.5">
        {citations.map((c, i) => (
          <li key={i}>
            {c.url ? (
              <a
                href={c.url}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-sm text-gblue transition hover:bg-gblue-soft/50 hover:underline"
              >
                <LinkIcon width={14} height={14} className="shrink-0" />
                <span className="truncate">{c.title}</span>
              </a>
            ) : (
              <span className="inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-sm text-ink-soft">
                <span className="truncate">{c.title}</span>
              </span>
            )}
            {c.snippet && (
              <p className="mt-0.5 pl-9 text-xs leading-relaxed text-ink-faint">
                {c.snippet}
              </p>
            )}
          </li>
        ))}
      </ul>
    </div>
  );
}
