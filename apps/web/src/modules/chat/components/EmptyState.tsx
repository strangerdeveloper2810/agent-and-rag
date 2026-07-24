import { useEffect, useState } from "react";
import SuggestionChips from "@/shared/components/SuggestionChips";

async function fetchSuggestions(): Promise<string[]> {
  try {
    const baseUrl = import.meta.env.VITE_AGENT_URL ?? "";
    const res = await fetch(`${baseUrl}/suggestions`);
    if (!res.ok) throw new Error("failed");
    const data = await res.json();
    if (data.suggestions?.length) return data.suggestions;
    throw new Error("empty");
  } catch {
    return [
      "Analyze the latest security reports",
      "Search documents for project specs",
      "Generate a summary of recent activity",
      "Translate this document to Vietnamese",
      "Research a technical topic in depth",
    ];
  }
}

function Skeleton() {
  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
      {Array.from({ length: 4 }).map((_, i) => (
        <div
          key={i}
          className="rounded-xl px-4 py-4 animate-pulse"
          style={{
            height: 52,
            animationDelay: `${i * 100}ms`,
            backgroundColor: "var(--bg-raised)",
          }}
        />
      ))}
    </div>
  );
}

export default function EmptyState({
  onPick,
}: {
  onPick: (prompt: string) => void;
}) {
  const [suggestions, setSuggestions] = useState<string[] | null>(null);

  useEffect(() => {
    fetchSuggestions().then(setSuggestions);
  }, []);

  return (
    <div className="mx-auto flex h-full max-w-3xl flex-col justify-center px-4 sm:px-6 animate-fade-in">
      {/* Centerpiece */}
      <div className="mb-6 flex flex-col items-center">
        {/* Glowing orb */}
        <div
          className="mb-5 flex h-20 w-20 items-center justify-center rounded-full"
          style={{
            background:
              "radial-gradient(circle, rgba(0,240,255,0.3) 0%, rgba(0,240,255,0.05) 60%, transparent 100%)",
            boxShadow:
              "0 0 40px rgba(0,240,255,0.2), 0 0 80px rgba(0,240,255,0.1)",
          }}
        >
          <svg
            width={32}
            height={32}
            viewBox="0 0 24 24"
            fill="none"
            stroke="var(--accent)"
            strokeWidth={1.4}
            strokeLinecap="round"
            strokeLinejoin="round"
            style={{
              filter: "drop-shadow(0 0 6px rgba(0,240,255,0.6))",
            }}
          >
            <path d="M12 4c.6 3.7 1.8 4.9 5.5 5.5-3.7.6-4.9 1.8-5.5 5.5-.6-3.7-1.8-4.9-5.5-5.5 3.7-.6 4.9-1.8 5.5-5.5Z" />
            <path d="M18 14c.3 1.6.8 2.1 2.4 2.4-1.6.3-2.1.8-2.4 2.4-.3-1.6-.8-2.1-2.4-2.4 1.6-.3 2.1-.8 2.4-2.4Z" />
          </svg>
        </div>

        <h1
          className="text-2xl font-medium tracking-[0.3em] uppercase animate-neon-pulse sm:text-3xl"
          style={{ color: "var(--accent)" }}
        >
          J.A.R.V.I.S.
        </h1>

        <div className="mt-1 flex items-center gap-2">
          <span className="relative flex h-1.5 w-1.5" aria-hidden="true">
            <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-[var(--success)] opacity-75" />
            <span className="relative inline-flex h-1.5 w-1.5 rounded-full bg-[var(--success)]" />
          </span>
          <span
            className="text-[11px] tracking-[0.2em] uppercase"
            style={{ color: "var(--text-tertiary)" }}
          >
            ONLINE
          </span>
        </div>

        <p
          className="mt-4 text-sm text-center"
          style={{ color: "var(--text-secondary)" }}
        >
          How can I help you today?
        </p>
      </div>

      {/* Suggestions */}
      <div className="mt-6">
        {suggestions === null ? (
          <Skeleton />
        ) : (
          <SuggestionChips suggestions={suggestions} onPick={onPick} />
        )}
      </div>
    </div>
  );
}
