import { useTranslation } from "react-i18next";
import { CheckCircleIcon } from "@heroicons/react/24/outline";

import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { useRevealOnScroll } from "@/modules/landing/hooks/useRevealOnScroll";

export interface FeatureDetailProps {
  /** Khoá i18n con trong `featuresPage.items.<itemKey>` (eyebrow/title/desc/badge/bullets). */
  itemKey: string;
  icon: React.ComponentType<React.SVGProps<SVGSVGElement>>;
  /** Đảo cột trái/phải để tạo layout zigzag khi lặp nhiều hàng liên tiếp. */
  reversed?: boolean;
  /** Tô nền bg-card/40 xen kẽ giữa các hàng, giống nhịp HowItWorks. */
  tinted?: boolean;
}

/**
 * FeatureDetail — 1 hàng chi tiết tính năng (text + bullet-list bên 1 cột, icon panel bên
 * cột kia). Tách component để LandingFeatures chỉ cần lặp 1 mảng cấu hình thay vì lặp JSX
 * cho từng nhóm tính năng — cùng tinh thần composition với FeatureGrid/HowItWorks đã có.
 */
export const FeatureDetail: React.FC<FeatureDetailProps> = ({
  itemKey,
  icon: Icon,
  reversed = false,
  tinted = false,
}) => {
  const { t } = useTranslation("landing");
  const { ref, isVisible } = useRevealOnScroll<HTMLDivElement>();
  const bullets = t(`featuresPage.items.${itemKey}.bullets`, {
    returnObjects: true,
  }) as string[];

  return (
    <section
      className={cn(
        "px-6 py-16 sm:py-20",
        tinted && "border-y border-border bg-card/40",
      )}
    >
      <div
        ref={ref}
        className={cn(
          "mx-auto grid max-w-6xl gap-10 sm:grid-cols-2 sm:items-center sm:gap-16",
          isVisible ? "animate-slide-up" : "opacity-0",
        )}
      >
        <div className={reversed ? "sm:order-2" : undefined}>
          <span className="text-xs font-mono font-bold uppercase tracking-widest text-primary">
            {t(`featuresPage.items.${itemKey}.eyebrow`)}
          </span>
          <h2 className="mt-3 font-display text-2xl font-extrabold tracking-tight text-foreground sm:text-3xl">
            {t(`featuresPage.items.${itemKey}.title`)}
          </h2>
          <p className="mt-4 text-sm leading-relaxed text-muted-foreground sm:text-base">
            {t(`featuresPage.items.${itemKey}.desc`)}
          </p>
          <ul className="mt-6 space-y-3">
            {bullets.map((bullet) => (
              <li
                key={bullet}
                className="flex items-start gap-2.5 text-sm text-foreground"
              >
                <CheckCircleIcon className="mt-0.5 h-4 w-4 shrink-0 text-primary" />
                <span>{bullet}</span>
              </li>
            ))}
          </ul>
        </div>

        <div className={reversed ? "sm:order-1" : undefined}>
          <div className="glass relative flex flex-col items-center justify-center gap-4 overflow-hidden rounded-3xl p-12 text-center">
            <div className="pointer-events-none absolute -bottom-10 left-1/2 h-48 w-48 -translate-x-1/2 rounded-full bg-primary/15 blur-[100px]" />
            <div className="relative z-10 flex h-16 w-16 items-center justify-center rounded-3xl bg-gradient-to-br from-indigo-500 to-purple-600 shadow-xl shadow-indigo-500/30">
              <Icon className="h-8 w-8 text-white" />
            </div>
            <Badge variant="accent" className="relative z-10 font-mono">
              {t(`featuresPage.items.${itemKey}.badge`)}
            </Badge>
          </div>
        </div>
      </div>
    </section>
  );
};

export default FeatureDetail;
