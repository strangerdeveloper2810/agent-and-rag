import { useEffect, useState, useMemo, useCallback } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import {
  SparklesIcon,
  DocumentTextIcon,
  CodeBracketIcon,
  MagnifyingGlassIcon,
  LightBulbIcon,
  CpuChipIcon,
  ArrowRightIcon,
  ArrowPathIcon,
  BoltIcon,
} from "@heroicons/react/24/outline";
import type { EmptyStateProps } from "@/types";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";

const PROMPT_CATEGORY_IDS = [
  "creative",
  "rag",
  "dev",
  "search",
  "productivity",
] as const;

const PROMPT_CATEGORY_ICONS: Record<
  (typeof PROMPT_CATEGORY_IDS)[number],
  React.ComponentType<{ className?: string }>
> = {
  creative: LightBulbIcon,
  rag: DocumentTextIcon,
  dev: CodeBracketIcon,
  search: MagnifyingGlassIcon,
  productivity: BoltIcon,
};

/** Builds the prompt-suggestion database (category label + question pool) for the current locale. */
const buildPromptDatabase = (t: TFunction<"chat">) =>
  PROMPT_CATEGORY_IDS.map((id) => ({
    id,
    category: t(`emptyState.categories.${id}`),
    icon: PROMPT_CATEGORY_ICONS[id],
    pool: t(`emptyState.prompts.${id}`, { returnObjects: true }) as string[],
  }));

const fetchSuggestionsApi = async (t: TFunction<"chat">): Promise<string[]> => {
  try {
    const baseUrl = import.meta.env.VITE_AGENT_URL ?? "";
    const res = await fetch(`${baseUrl}/suggestions?_t=${Date.now()}`);
    if (!res.ok) throw new Error("failed");
    const data = await res.json();
    if (data.suggestions?.length) return data.suggestions;
    throw new Error("empty");
  } catch {
    const randomPool = t("emptyState.randomPool", {
      returnObjects: true,
    }) as string[];
    return [...randomPool].sort(() => Math.random() - 0.5).slice(0, 5);
  }
};

/**
 * EmptyState — Giao diện khởi tạo cuộc trò chuyện với các gợi ý hoàn toàn Dynamic.
 */
export const EmptyState: React.FC<EmptyStateProps> = ({ onPick }) => {
  const { t } = useTranslation("chat");
  const [suggestions, setSuggestions] = useState<string[]>([]);
  const [loadingSuggestions, setLoadingSuggestions] = useState(false);
  const [activeTab, setActiveTab] = useState(0);
  const [pickedPrompt, setPickedPrompt] = useState<string | null>(null);
  const [refreshSeed, setRefreshSeed] = useState(0);

  const PROMPT_DATABASE = useMemo(() => buildPromptDatabase(t), [t]);

  // Lấy gợi ý từ Server API
  const loadSuggestions = useCallback(async () => {
    setLoadingSuggestions(true);
    try {
      const items = await fetchSuggestionsApi(t);
      setSuggestions(items);
    } finally {
      setLoadingSuggestions(false);
    }
  }, [t]);

  useEffect(() => {
    loadSuggestions();
  }, [loadSuggestions]);

  // Chọn ngẫu nhiên 2 câu hỏi từ pool của danh mục đang chọn mỗi khi chuyển tab hoặc bấm làm mới
  const currentPrompts = useMemo(() => {
    void refreshSeed;
    const currentCategory = PROMPT_DATABASE[activeTab] ?? PROMPT_DATABASE[0];
    const pool = [...currentCategory.pool];

    // Thuật toán xáo trộn Fisher-Yates dựa theo refreshSeed
    for (let i = pool.length - 1; i > 0; i--) {
      const j = Math.floor(Math.random() * (i + 1));
      [pool[i], pool[j]] = [pool[j], pool[i]];
    }
    return pool.slice(0, 2);
  }, [activeTab, refreshSeed, PROMPT_DATABASE]);

  const handlePick = (text: string) => {
    if (pickedPrompt) return;
    setPickedPrompt(text);
    onPick(text);
  };

  const handleRefreshAll = () => {
    setRefreshSeed((s) => s + 1);
    loadSuggestions();
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

        <Badge
          variant="accent"
          className="mb-2.5 gap-1.5 py-1 px-3 text-xs font-bold"
        >
          <SparklesIcon className="h-3.5 w-3.5" />
          <span>{t("emptyState.badge")}</span>
        </Badge>

        <h1 className="font-display text-2xl sm:text-3xl font-extrabold tracking-tight text-foreground">
          {t("emptyState.title")}
        </h1>
        <p className="mt-2 text-xs sm:text-sm max-w-lg leading-relaxed text-muted-foreground">
          {t("emptyState.subtitle")}
        </p>
      </div>

      {/* Categorized Smart Prompt Cards */}
      <div className="relative z-10 space-y-4">
        {/* Category Tabs with Dynamic Refresh Button */}
        <div className="flex flex-wrap items-center justify-center gap-2">
          {PROMPT_DATABASE.map((cat, idx) => {
            const Icon = cat.icon;
            const active = idx === activeTab;
            return (
              <Button
                key={cat.id}
                type="button"
                variant={active ? "secondary" : "outline"}
                onClick={() => {
                  setActiveTab(idx);
                  setRefreshSeed((s) => s + 1);
                }}
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

          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={handleRefreshAll}
            title={t("emptyState.refreshAllTitle")}
            className="gap-1.5 px-3 py-2 text-xs text-muted-foreground hover:text-primary rounded-xl transition-all"
          >
            <ArrowPathIcon
              className={`h-3.5 w-3.5 ${loadingSuggestions ? "animate-spin text-primary" : ""}`}
            />
            <span className="hidden sm:inline">{t("emptyState.refresh")}</span>
          </Button>
        </div>

        {/* Selected Dynamic Prompts List */}
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-1">
          {currentPrompts.map((promptText) => (
            <Card
              key={promptText}
              onClick={() => handlePick(promptText)}
              className={`group cursor-pointer flex items-center justify-between p-4 sm:p-4.5 transition-all duration-200 hover:border-primary hover:bg-primary/5 hover:shadow-md ${
                pickedPrompt === promptText
                  ? "opacity-60 border-primary bg-primary/10 pointer-events-none"
                  : ""
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

        {/* Dynamic Server Suggestions Badges */}
        {suggestions && suggestions.length > 0 && (
          <div className="pt-3 text-center">
            <div className="flex items-center justify-center gap-2 mb-2.5">
              <span className="text-[10.5px] font-extrabold uppercase tracking-wider text-muted-foreground/80">
                {t("emptyState.autoSuggestionsLabel")}
              </span>
              <button
                type="button"
                onClick={loadSuggestions}
                disabled={loadingSuggestions}
                title={t("emptyState.refreshAiAria")}
                aria-label={t("emptyState.refreshAiAria")}
                className="text-muted-foreground hover:text-primary transition p-0.5 rounded"
              >
                <ArrowPathIcon
                  className={`h-3 w-3 ${loadingSuggestions ? "animate-spin text-primary" : ""}`}
                />
              </button>
            </div>

            <div className="flex flex-wrap justify-center gap-2">
              {suggestions.slice(0, 4).map((sug) => (
                <Badge
                  key={sug}
                  variant="outline"
                  onClick={() => handlePick(sug)}
                  className={`cursor-pointer hover:border-primary hover:text-primary py-1 px-3 text-xs font-semibold transition shadow-sm bg-card/60 backdrop-blur-md ${
                    pickedPrompt === sug
                      ? "opacity-60 border-primary pointer-events-none"
                      : ""
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
