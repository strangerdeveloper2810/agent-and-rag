import React from "react";
import { useTranslation } from "react-i18next";
import { persistLocale, type Locale } from "@/i18n/locale";

/**
 * LanguageSwitcher — Segmented pill switch for VI / EN.
 * Explicitly displays both languages with active pill highlight,
 * eliminating any ambiguity on active vs target language.
 */
export const LanguageSwitcher: React.FC = () => {
  const { i18n } = useTranslation();
  const current: Locale = i18n.language === "en" ? "en" : "vi";

  const setLocale = (locale: Locale) => {
    if (current !== locale) {
      i18n.changeLanguage(locale);
      persistLocale(locale);
    }
  };

  return (
    <div
      role="group"
      aria-label="Language selection"
      className="inline-flex items-center rounded-full bg-muted/60 p-0.5 border border-border/50 text-[11px] font-medium tracking-wide transition-all shadow-xs backdrop-blur-sm"
    >
      <button
        type="button"
        onClick={() => setLocale("vi")}
        aria-pressed={current === "vi"}
        aria-label="Tiếng Việt"
        className={`relative flex items-center justify-center rounded-full px-2.5 py-1 transition-all duration-200 ${
          current === "vi"
            ? "bg-background text-foreground font-bold shadow-xs border border-border/40"
            : "text-muted-foreground hover:text-foreground hover:bg-background/40"
        }`}
      >
        VI
      </button>

      <button
        type="button"
        onClick={() => setLocale("en")}
        aria-pressed={current === "en"}
        aria-label="English"
        className={`relative flex items-center justify-center rounded-full px-2.5 py-1 transition-all duration-200 ${
          current === "en"
            ? "bg-background text-foreground font-bold shadow-xs border border-border/40"
            : "text-muted-foreground hover:text-foreground hover:bg-background/40"
        }`}
      >
        EN
      </button>
    </div>
  );
};

export default LanguageSwitcher;
