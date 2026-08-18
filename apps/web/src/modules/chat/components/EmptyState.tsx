import React, { useEffect, useState, useMemo, useCallback } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import {
  SparklesIcon,
  DocumentTextIcon,
  CodeBracketIcon,
  MagnifyingGlassIcon,
  LightBulbIcon,
  ArrowRightIcon,
  ArrowPathIcon,
  BoltIcon,
} from "@heroicons/react/24/outline";
import type { EmptyStateProps } from "@/types";
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

/** Builds the prompt-suggestion database for the current locale. */
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
    return [...randomPool].sort(() => Math.random() - 0.5).slice(0, 4);
  }
};

/**
 * EmptyState — Refined, Eye-Comfort, High-Legibility Initial Greeting View.
 */
export const EmptyState: React.FC<EmptyStateProps> = ({ onPick }) => {
  const { t } = useTranslation("chat");
  const [suggestions, setSuggestions] = useState<string[]>([]);
  const [loadingSuggestions, setLoadingSuggestions] = useState(false);
  const [activeTab, setActiveTab] = useState(0);
  const [pickedPrompt, setPickedPrompt] = useState<string | null>(null);
  const [refreshSeed, setRefreshSeed] = useState(0);

  const PROMPT_DATABASE = useMemo(() => buildPromptDatabase(t), [t]);

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

  const currentPrompts = useMemo(() => {
    void refreshSeed;
    const currentCategory = PROMPT_DATABASE[activeTab] ?? PROMPT_DATABASE[0];
    const pool = [...(currentCategory?.pool || [])];

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
    <div className="mx-auto flex h-full max-w-3xl flex-col justify-center px-4 sm:px-6 py-6 relative select-none animate-fade-in">
      {/* Soft Ambient Radial Background */}
      <div className="pointer-events-none absolute left-1/2 top-1/3 -translate-x-1/2 -translate-y-1/2 h-[380px] w-[380px] rounded-full bg-primary/8 blur-[100px]" />

      {/* Centerpiece Hero Greeting */}
      <div className="mb-6 flex flex-col items-center text-center relative z-10">
        {/* Sleek AI Glowing Avatar */}
        <div className="relative mb-3.5 flex h-14 w-14 items-center justify-center rounded-2xl bg-card border border-border/80 shadow-xs transition-transform duration-300 hover:scale-105">
          <div className="absolute -inset-0.5 rounded-2xl bg-gradient-to-tr from-primary/30 to-indigo-400/20 blur-xs" />
          <div className="relative flex items-center justify-center h-full w-full bg-card rounded-2xl">
            <SparklesIcon className="h-7 w-7 text-primary animate-pulse" />
          </div>
        </div>

        {/* Pill Tag */}
        <div className="inline-flex items-center gap-1.5 rounded-full px-3 py-1 bg-primary/10 border border-primary/20 text-primary text-xs font-semibold tracking-wide mb-2.5">
          <SparklesIcon className="h-3.5 w-3.5" />
          <span>{t("emptyState.badge")}</span>
        </div>

        <h1 className="font-display text-2xl sm:text-3xl font-bold tracking-tight text-foreground">
          {t("emptyState.title")}
        </h1>
        <p className="mt-2 text-sm sm:text-base max-w-lg leading-relaxed text-muted-foreground font-normal">
          {t("emptyState.subtitle")}
        </p>
      </div>

      {/* Categorized Smart Prompt Cards */}
      <div className="relative z-10 space-y-4">
        {/* Category Tabs */}
        <div className="flex flex-wrap items-center justify-center gap-2">
          {PROMPT_DATABASE.map((cat, idx) => {
            const Icon = cat.icon;
            const active = idx === activeTab;
            return (
              <button
                key={cat.id}
                type="button"
                onClick={() => {
                  setActiveTab(idx);
                  setRefreshSeed((s) => s + 1);
                }}
                className={`inline-flex items-center gap-2 px-3.5 py-2 text-xs sm:text-sm font-medium rounded-full transition-all duration-200 cursor-pointer shadow-xs ${
                  active
                    ? "bg-primary text-primary-foreground font-semibold ring-1 ring-primary/30"
                    : "bg-card border border-border/80 text-foreground/80 hover:text-foreground hover:border-primary/40 hover:bg-muted/60"
                }`}
              >
                <Icon className={`h-4 w-4 ${active ? "text-primary-foreground" : "text-primary"}`} />
                <span>{cat.category}</span>
              </button>
            );
          })}

          <button
            type="button"
            onClick={handleRefreshAll}
            title={t("emptyState.refreshAllTitle")}
            className="inline-flex items-center gap-1.5 px-3 py-2 text-xs sm:text-sm font-medium text-muted-foreground hover:text-primary transition-colors rounded-full hover:bg-muted/60 cursor-pointer"
          >
            <ArrowPathIcon
              className={`h-4 w-4 ${loadingSuggestions ? "animate-spin text-primary" : ""}`}
            />
            <span>{t("emptyState.refresh")}</span>
          </button>
        </div>

        {/* Selected Dynamic Prompts Cards (2 Main Cards) */}
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-1">
          {currentPrompts.map((promptText) => (
            <Card
              key={promptText}
              onClick={() => handlePick(promptText)}
              className={`group cursor-pointer flex items-center justify-between p-4 rounded-xl border border-border/80 bg-card transition-all duration-200 hover:border-primary/50 hover:bg-primary/[0.03] hover:shadow-sm ${
                pickedPrompt === promptText
                  ? "opacity-50 border-primary bg-primary/10 pointer-events-none"
                  : ""
              }`}
            >
              <span className="text-sm font-medium leading-normal text-foreground group-hover:text-primary transition-colors line-clamp-2 pr-2">
                {promptText}
              </span>
              <div className="flex h-7.5 w-7.5 shrink-0 items-center justify-center rounded-full bg-muted text-muted-foreground group-hover:bg-primary group-hover:text-primary-foreground transition-all duration-200">
                <ArrowRightIcon className="h-4 w-4 transition-transform duration-200 group-hover:translate-x-0.5" />
              </div>
            </Card>
          ))}
        </div>

        {/* Dynamic AI Suggested Pills (Bottom Section) */}
        {suggestions.length > 0 && (
          <div className="pt-2">
            <div className="flex items-center justify-center gap-1.5 mb-2.5 text-xs font-semibold tracking-normal text-muted-foreground">
              <SparklesIcon className="h-3.5 w-3.5 text-amber-500" />
              <span>{t("emptyState.autoSuggestionsLabel")}</span>
            </div>
            <div className="flex flex-wrap items-center justify-center gap-2">
              {suggestions.slice(0, 4).map((item) => (
                <button
                  key={item}
                  type="button"
                  onClick={() => handlePick(item)}
                  className="inline-flex items-center gap-2 rounded-full border border-border/80 bg-card px-3.5 py-1.5 text-xs sm:text-sm font-medium text-foreground hover:text-primary hover:border-primary/40 hover:bg-primary/[0.04] transition-all duration-200 shadow-xs cursor-pointer"
                >
                  <span className="text-amber-500">✨</span>
                  <span className="truncate max-w-[280px] sm:max-w-none">{item}</span>
                </button>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
};

export default EmptyState;
