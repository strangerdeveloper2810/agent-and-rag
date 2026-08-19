import { useTranslation } from "react-i18next";
import {
  UserPlusIcon,
  DocumentArrowUpIcon,
  ChatBubbleLeftRightIcon,
  CheckCircleIcon,
} from "@heroicons/react/24/outline";

import { cn } from "@/lib/utils";
import { useRevealOnScroll } from "@/modules/landing/hooks/useRevealOnScroll";

const STEP_ITEMS = [
  { key: "step1", icon: UserPlusIcon },
  { key: "step2", icon: DocumentArrowUpIcon },
  { key: "step3", icon: ChatBubbleLeftRightIcon },
  { key: "step4", icon: CheckCircleIcon },
] as const;

/**
 * HowItWorks — quy trình 4 bước, dùng số thứ tự + icon để tăng khả năng quét mắt (scannability).
 * Không tách card riêng để phân biệt rõ với FeatureGrid (feature = trụ cột, step = tiến trình tuyến tính).
 */
export const HowItWorks: React.FC = () => {
  const { t } = useTranslation("landing");
  const { ref, isVisible } = useRevealOnScroll<HTMLDivElement>();

  return (
    <section className="border-y border-border bg-card/40 px-6 py-24 sm:py-28">
      <div className="mx-auto max-w-6xl">
        <div className="mx-auto max-w-2xl text-center">
          <span className="text-xs font-mono font-bold uppercase tracking-widest text-primary">
            {t("howItWorks.eyebrow")}
          </span>
          <h2 className="mt-3 font-display text-3xl font-extrabold tracking-tight text-foreground sm:text-4xl">
            {t("howItWorks.title")}
          </h2>
          <p className="mt-4 text-sm leading-relaxed text-muted-foreground sm:text-base">
            {t("howItWorks.subtitle")}
          </p>
        </div>

        <div
          ref={ref}
          className="relative mt-16 grid gap-10 sm:grid-cols-2 lg:grid-cols-4"
        >
          {STEP_ITEMS.map(({ key, icon: Icon }, idx) => (
            <div
              key={key}
              style={
                isVisible ? { animationDelay: `${idx * 90}ms` } : undefined
              }
              className={cn(
                "relative flex flex-col items-center text-center",
                isVisible ? "animate-slide-up" : "opacity-0",
              )}
            >
              <div className="relative flex h-14 w-14 items-center justify-center rounded-2xl border border-border bg-background shadow-sm">
                <Icon className="h-6 w-6 text-primary" />
                <span className="absolute -right-2 -top-2 flex h-6 w-6 items-center justify-center rounded-full bg-primary text-[11px] font-bold text-primary-foreground shadow">
                  {idx + 1}
                </span>
              </div>
              <h3 className="mt-5 font-display text-sm font-bold tracking-tight text-foreground">
                {t(`howItWorks.steps.${key}.title`)}
              </h3>
              <p className="mt-2 text-xs leading-relaxed text-muted-foreground">
                {t(`howItWorks.steps.${key}.desc`)}
              </p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
};

export default HowItWorks;
