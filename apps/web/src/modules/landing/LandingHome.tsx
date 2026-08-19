import { useTranslation } from "react-i18next";

import { useDocumentTitle } from "@/hooks/useDocumentTitle";
import { LandingHeader } from "./components/LandingHeader";
import { Hero } from "./components/Hero";
import { FeatureGrid } from "./components/FeatureGrid";
import { HowItWorks } from "./components/HowItWorks";
import { CtaSection } from "./components/CtaSection";
import { LandingFooter } from "./components/LandingFooter";

/**
 * LandingHome — trang preview UI cho landing page public (chỉ dùng để visual review,
 * KHÔNG phải route thật cuối cùng — implementation đầy đủ theo
 * docs/plans/2026-08-19-landing-page-ssr-design.md (SSG/SEO/tách bundle) sẽ làm ở giai đoạn sau).
 *
 * Lưu ý: `#root`/`body` trong index.css bị đặt `overflow: hidden` toàn cục (phục vụ layout
 * chat cố định chiều cao). Landing page dài hơn 1 viewport nên cần tự bọc scroll riêng.
 */
export const LandingHome: React.FC = () => {
  const { t } = useTranslation("landing");
  useDocumentTitle(t("pageTitle"), t("pageDescription"));

  return (
    <div className="h-screen overflow-y-auto scroll-fine bg-background text-foreground">
      <LandingHeader />
      <main>
        <Hero />
        <FeatureGrid />
        <HowItWorks />
        <CtaSection />
      </main>
      <LandingFooter />
    </div>
  );
};

export default LandingHome;
