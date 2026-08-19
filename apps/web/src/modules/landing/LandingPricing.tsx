import { useTranslation } from "react-i18next";

import { useLandingDocumentTitle } from "./hooks/useLandingDocumentTitle";
import { LandingHeader } from "./components/LandingHeader";
import { PricingTiers } from "./components/PricingTiers";
import { PricingComparison } from "./components/PricingComparison";
import { PricingFaq } from "./components/PricingFaq";
import { CtaSection } from "./components/CtaSection";
import { LandingFooter } from "./components/LandingFooter";

/**
 * LandingPricing — trang "/pricing". Dự án đang early-access, KHÔNG có mô hình giá cố định
 * đã chốt, nên trang này dùng 2 "gói" theo use-case (Cá nhân/Đội nhóm — xem PricingTiers)
 * với CTA "đăng ký/liên hệ quan tâm" thay vì số tiền cụ thể, kèm bảng so sánh tính năng
 * (PricingComparison) và FAQ riêng cho giá (PricingFaq).
 * Xem docs/plans/2026-08-19-landing-page-ssr-design.md — mục "Nội dung landing".
 */
export const LandingPricing: React.FC = () => {
  const { t } = useTranslation("landing");
  // Khớp đúng logic buildHeadTags() trong scripts/prerender.mjs (trang không
  // phải home thì title = "{pageTitle} — J.A.R.V.I.S.").
  useLandingDocumentTitle(
    `${t("pricing.pageTitle")} — J.A.R.V.I.S.`,
    t("pricing.pageDescription"),
  );

  return (
    <div className="h-screen overflow-y-auto scroll-fine bg-background text-foreground">
      <LandingHeader slug="pricing" />
      <main>
        <section className="relative overflow-hidden px-6 pb-4 pt-16 sm:pt-24">
          <div className="pointer-events-none absolute -left-24 top-10 h-[420px] w-[420px] rounded-full bg-primary/10 blur-[130px]" />

          <div className="relative z-10 mx-auto max-w-3xl text-center">
            <span className="animate-fade-in text-xs font-mono font-bold uppercase tracking-widest text-primary">
              {t("pricing.hero.eyebrow")}
            </span>
            <h1 className="animate-slide-up mt-3 font-display text-4xl font-extrabold leading-tight tracking-tight text-foreground sm:text-5xl">
              {t("pricing.hero.title")}
            </h1>
            <p className="animate-slide-up mt-5 text-sm leading-relaxed text-muted-foreground sm:text-base">
              {t("pricing.hero.subtitle")}
            </p>
          </div>
        </section>

        <PricingTiers />
        <PricingComparison />
        <PricingFaq />
        <CtaSection />
      </main>
      <LandingFooter />
    </div>
  );
};

export default LandingPricing;
