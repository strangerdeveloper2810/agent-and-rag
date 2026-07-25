import { useEffect, useState } from "react";
import SuggestionChips from "@/shared/components/SuggestionChips";

import type { EmptyStateProps } from "@/types";

/**
 * Fetches initial prompt recommendations from server or returns fallbacks.
 *
 * @returns Promise resolving to array of prompt suggestion strings
 */
const fetchSuggestions = async (): Promise<string[]> => {
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
};

/**
 * Skeleton loader component for suggestion chips.
 */
const Skeleton: React.FC = () => {
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
};

/**
 * EmptyState centerpiece component rendered when starting a new chat conversation.
 */
export const EmptyState: React.FC<EmptyStateProps> = ({ onPick }) => {
  const [suggestions, setSuggestions] = useState<string[] | null>(null);

  useEffect(() => {
    fetchSuggestions().then(setSuggestions);
  }, []);

  return (
    <div className="mx-auto flex h-full max-w-3xl flex-col justify-center px-4 sm:px-6 animate-fade-in py-8 relative">
      {/* Background Raycast Ambient Glow */}
      <div 
        className="pointer-events-none absolute left-1/2 top-1/3 -translate-x-1/2 -translate-y-1/2 h-72 w-72 rounded-full blur-[120px] opacity-20"
        style={{ background: "radial-gradient(circle, var(--accent) 0%, var(--accent-violet) 100%)" }}
      />

      {/* Centerpiece */}
      <div className="mb-8 flex flex-col items-center text-center relative z-10">
        <div
          className="mb-5 flex h-16 w-16 items-center justify-center rounded-2xl border shadow-lg transition-transform duration-300 hover:scale-105 backdrop-blur-xl glow-cyan-border"
          style={{
            borderColor: "rgba(59, 130, 246, 0.3)",
            backgroundColor: "var(--surface)",
          }}
        >
          <svg
            width={30}
            height={30}
            viewBox="0 0 24 24"
            fill="none"
            stroke="var(--accent)"
            strokeWidth={1.8}
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <path d="M12 3.5c.6 3.7 1.8 4.9 5.5 5.5-3.7.6-4.9 1.8-5.5 5.5-.6-3.7-1.8-4.9-5.5-5.5 3.7-.6 4.9-1.8 5.5-5.5Z" />
            <path d="M18 14c.3 1.6.8 2.1 2.4 2.4-1.6.3-2.1.8-2.4 2.4-.3-1.6-.8-2.1-2.4-2.4 1.6-.3 2.1-.8 2.4-2.4Z" />
          </svg>
        </div>

        <h1
          className="font-display text-2xl sm:text-3xl font-extrabold tracking-tight"
          style={{ color: "var(--text)" }}
        >
          How can J.A.R.V.I.S. help you today?
        </h1>
        <p
          className="mt-2.5 text-xs sm:text-sm font-medium max-w-md"
          style={{ color: "var(--text-secondary)" }}
        >
          Dispatch commands, search vector knowledge base, or execute intelligent workflows.
        </p>
      </div>

      {/* Suggestion Chips */}
      <div className="mt-2 relative z-10">
        {suggestions === null ? (
          <Skeleton />
        ) : (
          <SuggestionChips suggestions={suggestions} onPick={onPick} />
        )}
      </div>
    </div>
  );
};

export default EmptyState;
