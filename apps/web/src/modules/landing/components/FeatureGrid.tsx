import { useTranslation } from "react-i18next";
import {
  MagnifyingGlassIcon,
  CpuChipIcon,
  BoltIcon,
} from "@heroicons/react/24/outline";

import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { useRevealOnScroll } from "@/modules/landing/hooks/useRevealOnScroll";

const FEATURE_ITEMS = [
  { key: "rag", icon: MagnifyingGlassIcon },
  { key: "multiAgent", icon: CpuChipIcon },
  { key: "toolSkills", icon: BoltIcon },
] as const;

/**
 * FeatureGrid — 3 trụ cột tính năng (RAG, multi-agent routing, tool skills).
 * Composition: Card (atom có sẵn) + icon semantics theo từng feature, không tự vẽ card mới.
 */
export const FeatureGrid: React.FC = () => {
  const { t } = useTranslation("landing");
  const { ref, isVisible } = useRevealOnScroll<HTMLDivElement>();

  return (
    <section className="px-6 py-24 sm:py-28">
      <div className="mx-auto max-w-6xl">
        <div className="mx-auto max-w-2xl text-center">
          <span className="text-xs font-mono font-bold uppercase tracking-widest text-primary">
            {t("features.eyebrow")}
          </span>
          <h2 className="mt-3 font-display text-3xl font-extrabold tracking-tight text-foreground sm:text-4xl">
            {t("features.title")}
          </h2>
          <p className="mt-4 text-sm leading-relaxed text-muted-foreground sm:text-base">
            {t("features.subtitle")}
          </p>
        </div>

        <div
          ref={ref}
          className="mt-16 grid gap-6 sm:grid-cols-2 lg:grid-cols-3"
        >
          {FEATURE_ITEMS.map(({ key, icon: Icon }, idx) => (
            <Card
              key={key}
              style={
                isVisible ? { animationDelay: `${idx * 90}ms` } : undefined
              }
              className={cn(
                "group p-7 transition-all duration-200 hover:-translate-y-1 hover:border-primary/40 hover:shadow-lg",
                isVisible ? "animate-slide-up" : "opacity-0",
              )}
            >
              <div className="flex h-11 w-11 items-center justify-center rounded-2xl bg-primary/10 text-primary transition-colors group-hover:bg-primary group-hover:text-primary-foreground">
                <Icon className="h-5 w-5" />
              </div>
              <h3 className="mt-5 font-display text-lg font-bold tracking-tight text-foreground">
                {t(`features.items.${key}.title`)}
              </h3>
              <p className="mt-2.5 text-sm leading-relaxed text-muted-foreground">
                {t(`features.items.${key}.desc`)}
              </p>
              <Badge variant="accent" className="mt-5 font-mono">
                {t(`features.items.${key}.badge`)}
              </Badge>
            </Card>
          ))}
        </div>
      </div>
    </section>
  );
};

export default FeatureGrid;
