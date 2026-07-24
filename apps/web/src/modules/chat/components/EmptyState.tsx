import { useEffect, useState } from "react";
import SuggestionChips from "@/shared/components/SuggestionChips";

function getGreeting(): string {
  const hour = new Date().getHours();
  if (hour < 12) return "Good morning";
  if (hour < 17) return "Good afternoon";
  return "Good evening";
}

async function fetchSuggestions(): Promise<string[]> {
  try {
    // Go agent direct (dev) or via API gateway (prod)
    const baseUrl = import.meta.env.VITE_AGENT_URL ?? "";
    const res = await fetch(`${baseUrl}/suggestions`);
    if (!res.ok) throw new Error("failed");
    const data = await res.json();
    if (data.suggestions?.length) return data.suggestions;
    throw new Error("empty");
  } catch {
    // Fallback tối giản — LLM sẽ generate khi gọi lại
    return [
      "Hôm nay có việc gì cần giúp không?",
      "Tìm kiếm tài liệu gần đây",
      "Giải thích một khái niệm kỹ thuật",
      "Tạo task mới cho dự án",
      "Nghiên cứu một chủ đề bất kỳ",
      "Dịch văn bản sang ngôn ngữ khác",
    ];
  }
}

function Skeleton() {
  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
      {Array.from({ length: 6 }).map((_, i) => (
        <div
          key={i}
          className="rounded-2xl bg-subtle px-4 py-4 animate-pulse"
          style={{ height: 56, animationDelay: `${i * 100}ms` }}
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
  const greeting = getGreeting();

  useEffect(() => {
    fetchSuggestions().then(setSuggestions);
  }, []);

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
        {greeting}. How can I help you today?
      </p>

      <div className="mt-10">
        {suggestions === null ? (
          <Skeleton />
        ) : (
          <SuggestionChips suggestions={suggestions} onPick={onPick} />
        )}
      </div>
    </div>
  );
}
