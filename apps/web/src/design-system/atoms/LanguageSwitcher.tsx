import { useTranslation } from "react-i18next";
import { persistLocale, type Locale } from "@/i18n/locale";

/**
 * LanguageSwitcher — toggles UI language (vi/en), mirrors ThemeToggle's
 * "shows the target state" convention: label displays the language you'll
 * switch TO, not the current one.
 */
export const LanguageSwitcher: React.FC = () => {
  const { t, i18n } = useTranslation();
  const current: Locale = i18n.language === "en" ? "en" : "vi";
  const target: Locale = current === "vi" ? "en" : "vi";

  const toggle = () => {
    i18n.changeLanguage(target);
    persistLocale(target);
  };

  return (
    <button
      onClick={toggle}
      aria-label={t("language.toggle")}
      className="flex items-center gap-2 rounded-full px-3 py-1.5 text-xs transition hover:bg-muted text-muted-foreground hover:text-foreground"
    >
      <span className="hidden sm:inline tracking-wider uppercase text-[10px]">
        {target.toUpperCase()}
      </span>
    </button>
  );
};

export default LanguageSwitcher;
