import { useTranslation } from "react-i18next";
import {
  CpuChipIcon,
  SparklesIcon,
  CommandLineIcon,
  CubeTransparentIcon,
} from "@heroicons/react/24/outline";

import { cn } from "@/lib/utils";
import { useRevealOnScroll } from "@/modules/landing/hooks/useRevealOnScroll";

const PROVIDERS = [
  { key: "deepseek", icon: CpuChipIcon },
  { key: "gemini", icon: SparklesIcon },
  { key: "claude", icon: CommandLineIcon },
  { key: "langgraph", icon: CubeTransparentIcon },
] as const;

/**
 * TechStack — dải "powered by" liệt kê các nhà cung cấp AI/framework thật đang dùng
 * (khớp danh sách trong features.multiAgent + JSON-LD featureList của index.html).
 */
export const TechStack: React.FC = () => {
  const { t } = useTranslation("landing");
  const { ref, isVisible } = useRevealOnScroll<HTMLDivElement>();

  return (
    <section className="px-6 py-16">
      <div className="mx-auto max-w-5xl text-center">
        <span className="text-xs font-mono font-bold uppercase tracking-widest text-primary">
          {t("techStack.eyebrow")}
        </span>
        <h2 className="mt-3 font-display text-xl font-bold tracking-tight text-foreground sm:text-2xl">
          {t("techStack.title")}
        </h2>

        <div
          ref={ref}
          className={cn(
            "mt-9 flex flex-wrap items-center justify-center gap-3",
            isVisible ? "animate-fade-in" : "opacity-0",
          )}
        >
          {PROVIDERS.map(({ key, icon: Icon }) => (
            <div
              key={key}
              className="flex items-center gap-2 rounded-full border border-border/60 bg-card/60 px-4 py-2 text-sm font-semibold text-muted-foreground transition-colors hover:border-primary/40 hover:text-foreground"
            >
              <Icon className="h-4 w-4 text-primary" />
              {t(`techStack.providers.${key}`)}
            </div>
          ))}
        </div>
      </div>
    </section>
  );
};

export default TechStack;
