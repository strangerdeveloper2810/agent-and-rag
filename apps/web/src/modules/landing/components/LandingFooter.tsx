import { useTranslation } from "react-i18next";
import { SparklesIcon } from "@heroicons/react/24/outline";

/**
 * LandingFooter — footer tối giản, nhất quán brand mark với header.
 */
export const LandingFooter: React.FC = () => {
  const { t } = useTranslation("landing");

  return (
    <footer className="border-t border-border px-6 py-8">
      <div className="mx-auto flex max-w-6xl flex-col items-center justify-between gap-3 text-center sm:flex-row sm:text-left">
        <div className="flex items-center gap-2 text-muted-foreground">
          <div className="flex h-6 w-6 items-center justify-center rounded-lg bg-gradient-to-br from-indigo-500 to-purple-600">
            <SparklesIcon className="h-3.5 w-3.5 text-white" />
          </div>
          <span className="text-xs">{t("footer.copyright")}</span>
        </div>
        <span className="text-[11px] font-mono uppercase tracking-widest text-muted-foreground">
          {t("footer.tagline")}
        </span>
      </div>
    </footer>
  );
};

export default LandingFooter;
