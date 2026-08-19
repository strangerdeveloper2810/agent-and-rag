import { useTranslation } from "react-i18next";
import {
  UserIcon,
  UserGroupIcon,
  CheckCircleIcon,
} from "@heroicons/react/24/outline";

import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { useRevealOnScroll } from "@/modules/landing/hooks/useRevealOnScroll";

const TIER_ITEMS = [
  {
    key: "individual",
    icon: UserIcon,
    ctaVariant: "outline" as const,
    highlighted: false,
  },
  {
    key: "team",
    icon: UserGroupIcon,
    ctaVariant: "gradient" as const,
    highlighted: true,
  },
] as const;

/**
 * PricingTiers — 2 "gói" phân theo use-case (Cá nhân / Đội nhóm), KHÔNG có số tiền cụ thể
 * (dự án đang early-access, chưa chốt mô hình giá). CTA điều hướng về /register — route thật
 * duy nhất hiện có để "đăng ký quan tâm" — cố tình KHÔNG bịa route /contact chưa tồn tại.
 */
export const PricingTiers: React.FC = () => {
  const { t } = useTranslation("landing");
  const { ref, isVisible } = useRevealOnScroll<HTMLDivElement>();

  return (
    <section className="px-6 py-24 sm:py-28">
      <div className="mx-auto max-w-6xl">
        <div className="mx-auto max-w-2xl text-center">
          <span className="text-xs font-mono font-bold uppercase tracking-widest text-primary">
            {t("pricing.tiers.eyebrow")}
          </span>
          <h2 className="mt-3 font-display text-3xl font-extrabold tracking-tight text-foreground sm:text-4xl">
            {t("pricing.tiers.title")}
          </h2>
          <p className="mt-4 text-sm leading-relaxed text-muted-foreground sm:text-base">
            {t("pricing.tiers.subtitle")}
          </p>
        </div>

        <div
          ref={ref}
          className="mx-auto mt-16 grid max-w-4xl gap-6 sm:grid-cols-2"
        >
          {TIER_ITEMS.map(
            ({ key, icon: Icon, ctaVariant, highlighted }, idx) => {
              const bullets = t(`pricing.tiers.items.${key}.bullets`, {
                returnObjects: true,
              }) as string[];

              return (
                <Card
                  key={key}
                  style={
                    isVisible ? { animationDelay: `${idx * 90}ms` } : undefined
                  }
                  className={cn(
                    "flex flex-col p-8 transition-all duration-200",
                    highlighted &&
                      "border-primary/50 shadow-lg shadow-primary/10",
                    isVisible ? "animate-slide-up" : "opacity-0",
                  )}
                >
                  <div className="flex h-11 w-11 items-center justify-center rounded-2xl bg-primary/10 text-primary">
                    <Icon className="h-5 w-5" />
                  </div>

                  <div className="mt-5 flex items-center gap-2">
                    <h3 className="font-display text-xl font-bold tracking-tight text-foreground">
                      {t(`pricing.tiers.items.${key}.name`)}
                    </h3>
                    {highlighted && (
                      <Badge variant="accent">
                        {t(`pricing.tiers.items.${key}.badge`)}
                      </Badge>
                    )}
                  </div>
                  <p className="mt-2 text-sm leading-relaxed text-muted-foreground">
                    {t(`pricing.tiers.items.${key}.tagline`)}
                  </p>

                  <ul className="mt-6 flex-1 space-y-3">
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

                  <a
                    href="/register"
                    className={cn(
                      buttonVariants({ variant: ctaVariant, size: "lg" }),
                      "mt-8 w-full",
                    )}
                  >
                    {t(`pricing.tiers.items.${key}.cta`)}
                  </a>
                </Card>
              );
            },
          )}
        </div>
      </div>
    </section>
  );
};

export default PricingTiers;
