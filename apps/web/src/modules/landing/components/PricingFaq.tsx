import { useTranslation } from "react-i18next";
import { ChevronDownIcon } from "@heroicons/react/24/outline";

import { cn } from "@/lib/utils";
import { useRevealOnScroll } from "@/modules/landing/hooks/useRevealOnScroll";

const PRICING_FAQ_KEYS = ["q1", "q2", "q3", "q4"] as const;

/**
 * PricingFaq — cùng pattern <details>/<summary> với Faq.tsx (landing Home), nội dung riêng
 * cho câu hỏi liên quan tới giá/early-access.
 */
export const PricingFaq: React.FC = () => {
  const { t } = useTranslation("landing");
  const { ref, isVisible } = useRevealOnScroll<HTMLDivElement>();

  return (
    <section className="px-6 py-24 sm:py-28">
      <div className="mx-auto max-w-3xl">
        <div className="mx-auto max-w-2xl text-center">
          <span className="text-xs font-mono font-bold uppercase tracking-widest text-primary">
            {t("pricing.faq.eyebrow")}
          </span>
          <h2 className="mt-3 font-display text-3xl font-extrabold tracking-tight text-foreground sm:text-4xl">
            {t("pricing.faq.title")}
          </h2>
        </div>

        <div
          ref={ref}
          className={cn(
            "mt-14 divide-y divide-border rounded-2xl border border-border bg-card/40",
            isVisible ? "animate-slide-up" : "opacity-0",
          )}
        >
          {PRICING_FAQ_KEYS.map((key) => (
            <details key={key} className="group px-6 py-5 open:pb-6">
              <summary className="flex cursor-pointer list-none items-center justify-between gap-4 font-display text-sm font-bold text-foreground sm:text-base [&::-webkit-details-marker]:hidden">
                {t(`pricing.faq.items.${key}.question`)}
                <ChevronDownIcon className="h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200 group-open:rotate-180" />
              </summary>
              <p className="mt-3 text-sm leading-relaxed text-muted-foreground">
                {t(`pricing.faq.items.${key}.answer`)}
              </p>
            </details>
          ))}
        </div>
      </div>
    </section>
  );
};

export default PricingFaq;
