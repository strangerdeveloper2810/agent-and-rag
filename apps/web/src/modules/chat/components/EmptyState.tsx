import { useEffect, useState } from "react";
import {
  SparklesIcon,
  DocumentTextIcon,
  CodeBracketIcon,
  MagnifyingGlassIcon,
  LightBulbIcon,
  CpuChipIcon,
  ArrowRightIcon,
} from "@heroicons/react/24/outline";
import type { EmptyStateProps } from "@/types";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";

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
      "Phân tích báo cáo an ninh mạng và rủi ro gần đây",
      "Tra cứu tài liệu quy định và tiêu chuẩn kỹ thuật",
      "Tóm tắt các hoạt động nổi bật trong tuần qua",
      "Dịch tài liệu thiết kế sang Tiếng Việt",
      "Nghiên cứu kiến trúc AI Agent và RAG Vector DB",
    ];
  }
};

const CATEGORIZED_PROMPTS = [
  {
    category: "Ý tưởng & Kế hoạch",
    icon: LightBulbIcon,
    badge: "Creative",
    items: [
      "Lập kế hoạch triển khai tính năng AI Agent cho doanh nghiệp",
      "Gợi ý 5 ý tưởng cải thiện trải nghiệm người dùng UI/UX",
    ],
  },
  {
    category: "Đọc & Phân tích tài liệu",
    icon: DocumentTextIcon,
    badge: "RAG",
    items: [
      "Phân tích và trích xuất điểm chính từ file PDF/Word",
      "So sánh thông số kỹ thuật giữa các phiên bản mô hình",
    ],
  },
  {
    category: "Lập trình & Clean Code",
    icon: CodeBracketIcon,
    badge: "Dev",
    items: [
      "Viết React Custom Hook quản lý WebSocket SSE Stream",
      "Tối ưu hóa truy vấn MongoDB và Vector Indexing",
    ],
  },
  {
    category: "Tra cứu Tri thức",
    icon: MagnifyingGlassIcon,
    badge: "Search",
    items: [
      "Tra cứu các quy trình chuẩn trong Knowledge Base",
      "Giải thích thuật toán Hybrid Search (Dense + Sparse)",
    ],
  },
];

/**
 * EmptyState — Perfectly balanced Shadcn UI centerpiece.
 */
export const EmptyState: React.FC<EmptyStateProps> = ({ onPick }) => {
  const [suggestions, setSuggestions] = useState<string[] | null>(null);
  const [activeTab, setActiveTab] = useState(0);
  const [pickedPrompt, setPickedPrompt] = useState<string | null>(null);

  useEffect(() => {
    fetchSuggestions().then(setSuggestions);
  }, []);

  const handlePick = (text: string) => {
    if (pickedPrompt) return;
    setPickedPrompt(text);
    onPick(text);
  };

  return (
    <div className="mx-auto flex h-full max-w-4xl flex-col justify-center px-4 sm:px-8 py-6 relative animate-fade-in">
      {/* Ambient background glow */}
      <div className="pointer-events-none absolute left-1/2 top-1/4 -translate-x-1/2 -translate-y-1/2 h-[420px] w-[420px] rounded-full bg-primary/10 blur-[130px]" />

      {/* Centerpiece AI Hologram Header */}
      <div className="mb-6 flex flex-col items-center text-center relative z-10">
        <div className="relative mb-4 flex h-18 w-18 items-center justify-center rounded-2xl bg-card border border-primary/30 shadow-xl transition-all duration-300 hover:scale-105 group p-3">
          <div className="absolute -inset-1 rounded-2xl bg-gradient-to-br from-indigo-500 to-purple-600 opacity-20 blur-md group-hover:opacity-40 transition duration-500" />
          
          <div className="relative flex items-center justify-center h-full w-full bg-card rounded-2xl border border-border">
            <CpuChipIcon className="h-9 w-9 text-primary animate-float-slow" />
            <SparklesIcon className="absolute -top-1 -right-1 h-5 w-5 text-amber-400 animate-pulse" />
          </div>
        </div>

        <Badge variant="accent" className="mb-2.5 gap-1.5 py-1 px-3 text-xs font-bold">
          <SparklesIcon className="h-3.5 w-3.5" />
          <span>J.A.R.V.I.S. Core Intelligence</span>
        </Badge>

        <h1 className="font-display text-2xl sm:text-3xl font-extrabold tracking-tight text-foreground">
          Tôi có thể giúp gì cho bạn hôm nay?
        </h1>
        <p className="mt-2 text-xs sm:text-sm max-w-lg leading-relaxed text-muted-foreground">
          Bắt đầu trò chuyện, yêu cầu tra cứu tài liệu doanh nghiệp hoặc thực thi quy trình làm việc thông minh.
        </p>
      </div>

      {/* Categorized Smart Prompt Cards */}
      <div className="relative z-10 space-y-4">
        {/* Category Tabs */}
        <div className="flex flex-wrap items-center justify-center gap-2">
          {CATEGORIZED_PROMPTS.map((cat, idx) => {
            const Icon = cat.icon;
            const active = idx === activeTab;
            return (
              <Button
                key={cat.category}
                type="button"
                variant={active ? "secondary" : "outline"}
                onClick={() => setActiveTab(idx)}
                className={`gap-2 px-3.5 py-2 text-xs sm:text-sm font-semibold rounded-xl transition-all ${
                  active
                    ? "border-primary bg-primary/10 text-primary shadow-sm ring-1 ring-primary/20 font-bold"
                    : "hover:bg-muted text-foreground"
                }`}
              >
                <Icon className="h-4 w-4 text-primary" />
                <span>{cat.category}</span>
              </Button>
            );
          })}
        </div>

        {/* Selected Prompts List */}
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-1">
          {CATEGORIZED_PROMPTS[activeTab].items.map((promptText) => (
            <Card
              key={promptText}
              onClick={() => handlePick(promptText)}
              className={`group cursor-pointer flex items-center justify-between p-4 sm:p-4.5 transition-all duration-200 hover:border-primary hover:bg-primary/5 hover:shadow-md ${
                pickedPrompt === promptText ? "opacity-60 border-primary bg-primary/10 pointer-events-none" : ""
              }`}
            >
              <div className="min-w-0 flex-1 pr-3">
                <p className="text-xs sm:text-sm font-bold text-foreground leading-relaxed group-hover:text-primary transition">
                  {promptText}
                </p>
              </div>
              <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground group-hover:bg-primary group-hover:text-primary-foreground transition-all shadow-sm">
                <ArrowRightIcon className="h-3.5 w-3.5" />
              </div>
            </Card>
          ))}
        </div>

        {/* Dynamic Server Suggestions */}
        {suggestions && suggestions.length > 0 && (
          <div className="pt-3 text-center">
            <p className="text-[10.5px] font-extrabold uppercase tracking-wider text-muted-foreground/80 mb-2.5">
              GỢI Ý BỔ SUNG TỪ HỆ THỐNG
            </p>
            <div className="flex flex-wrap justify-center gap-2">
              {suggestions.slice(0, 3).map((sug) => (
                <Badge
                  key={sug}
                  variant="outline"
                  onClick={() => handlePick(sug)}
                  className={`cursor-pointer hover:border-primary hover:text-primary py-1 px-3 text-xs font-semibold transition shadow-sm bg-card/60 backdrop-blur-md ${
                    pickedPrompt === sug ? "opacity-60 border-primary pointer-events-none" : ""
                  }`}
                >
                  ✨ {sug}
                </Badge>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
};

export default EmptyState;
