import { useMemo } from "react";
import SuggestionChips from "@/shared/components/SuggestionChips";

const SUGGESTION_POOL = {
  morning: [
    "Tổng hợp lịch hôm nay",
    "Briefing buổi sáng",
    "Có email quan trọng nào không?",
    "Thời tiết hôm nay thế nào?",
    "Việc cần làm hôm nay",
  ],
  afternoon: [
    "Tạo task cho buổi họp chiều",
    "Tìm tài liệu về dự án X",
    "Review code gần đây",
    "So sánh Go và Rust performance",
    "Tóm tắt ghi chú hôm nay",
  ],
  evening: [
    "Tổng kết công việc hôm nay",
    "Chuẩn bị standup sáng mai",
    "Có tin tức gì mới không?",
    "Nghiên cứu về AI agent architecture",
    "Lên kế hoạch ngày mai",
  ],
  technical: [
    "Explain how RAG works",
    "So sánh Gemini vs Claude cho code review",
    "Giải thích goroutine và channel trong Go",
    "Thiết kế REST API cho task management",
    "Làm sao để optimize token usage?",
    "MCP protocol hoạt động thế nào?",
  ],
  general: [
    "Dịch đoạn văn này sang tiếng Việt",
    "Viết email phản hồi khách hàng",
    "Tóm tắt bài báo này",
    "Lên ý tưởng cho feature mới",
    "Tạo ghi chú từ cuộc họp",
  ],
  creative: [
    "Brainstorm tên cho dự án mới",
    "Viết tweet về AI agents",
    "Thiết kế architecture cho chatbot",
    "Tạo proposal cho tính năng X",
    "Viết blog post về Go concurrency",
  ],
};

function getTimeOfDay(): "morning" | "afternoon" | "evening" {
  const hour = new Date().getHours();
  if (hour < 12) return "morning";
  if (hour < 17) return "afternoon";
  return "evening";
}

function getGreeting(): string {
  const hour = new Date().getHours();
  if (hour < 12) return "Good morning";
  if (hour < 17) return "Good afternoon";
  return "Good evening";
}

function shuffleAndPick<T>(arr: T[], count: number): T[] {
  const shuffled = [...arr];
  for (let i = shuffled.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [shuffled[i], shuffled[j]] = [shuffled[j], shuffled[i]];
  }
  return shuffled.slice(0, count);
}

function generateSuggestions(): string[] {
  const timeOfDay = getTimeOfDay();

  // Pick from each category, then shuffle for variety
  const timeSuggestions = SUGGESTION_POOL[timeOfDay];
  const fromTime = shuffleAndPick(timeSuggestions, 2);
  const fromTech = shuffleAndPick(SUGGESTION_POOL.technical, 2);
  const fromGeneral = shuffleAndPick(SUGGESTION_POOL.general, 1);
  const fromCreative = shuffleAndPick(SUGGESTION_POOL.creative, 1);

  // Combine and shuffle for mixed display
  return shuffleAndPick(
    [...fromTime, ...fromTech, ...fromGeneral, ...fromCreative],
    6
  );
}

export default function EmptyState({
  onPick,
}: {
  onPick: (prompt: string) => void;
}) {
  const suggestions = useMemo(() => generateSuggestions(), []);
  const greeting = getGreeting();

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
        <SuggestionChips suggestions={suggestions} onPick={onPick} />
      </div>
    </div>
  );
}
