import SuggestionChips from "@/shared/components/SuggestionChips";

const SUGGESTIONS = [
  "Explain how RAG works",
  "Create a task for team meeting on Friday",
  "What information is in the documents?",
  "How many documents are loaded?",
  "Search for recent notes",
  "Translate this to Vietnamese",
];

export default function EmptyState({
  onPick,
}: {
  onPick: (prompt: string) => void;
}) {
  return (
    <div className="mx-auto flex h-full max-w-3xl flex-col justify-center px-4 sm:px-6 animate-fade-in">
      <div className="mb-2 flex items-center gap-3">
        <div className="flex h-10 w-10 items-center justify-center rounded-full bg-gemini text-white">
          <svg
            width={20}
            height={20}
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth={1.6}
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <path d="M12 3.5c.6 3.7 1.8 4.9 5.5 5.5-3.7.6-4.9 1.8-5.5 5.5-.6-3.7-1.8-4.9-5.5-5.5 3.7-.6 4.9-1.8 5.5-5.5Z" />
            <path d="M18 14.5c.3 1.6.8 2.1 2.4 2.4-1.6.3-2.1.8-2.4 2.4-.3-1.6-.8-2.1-2.4-2.4 1.6-.3 2.1-.8 2.4-2.4Z" />
          </svg>
        </div>
        <h1 className="text-3xl font-medium tracking-tight sm:text-4xl">
          <span className="text-gemini">JARVIS</span>
        </h1>
      </div>
      <p className="mt-2 text-2xl font-medium text-ink-faint sm:text-3xl">
        How can I help you today?
      </p>

      <div className="mt-10">
        <SuggestionChips suggestions={SUGGESTIONS} onPick={onPick} />
      </div>
    </div>
  );
}
