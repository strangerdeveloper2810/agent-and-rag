import { useTranslation } from "react-i18next";

import { useLandingDocumentTitle } from "./hooks/useLandingDocumentTitle";
import { LandingHeader } from "./components/LandingHeader";
import { Hero } from "./components/Hero";
import { ProductPreview } from "./components/ProductPreview";
import { FeatureGrid } from "./components/FeatureGrid";
import { HowItWorks } from "./components/HowItWorks";
import { Faq } from "./components/Faq";
import { CtaSection } from "./components/CtaSection";
import { LandingFooter } from "./components/LandingFooter";

/**
 * LandingHome — trang chủ public ("/"), prerender tĩnh (SSG) lúc build — xem
 * scripts/prerender.mjs. Route hiển thị/nội dung SEO thật, không phải preview.
 *
 * Lưu ý: `#root`/`body` trong index.css bị đặt `overflow: hidden` toàn cục (phục vụ layout
 * chat cố định chiều cao). Landing page dài hơn 1 viewport nên cần tự bọc scroll riêng.
 */
export const LandingHome: React.FC = () => {
  const { t } = useTranslation("landing");
  // Không nối thêm brand ở đây — pageTitle đã là chuỗi đầy đủ, khớp với title
  // mà prerender.mjs chèn vào HTML tĩnh (xem useLandingDocumentTitle).
  useLandingDocumentTitle(t("pageTitle"), t("pageDescription"));

  return (
    <div className="h-screen overflow-y-auto scroll-fine bg-background text-foreground">
      <LandingHeader />
      <main>
        <Hero />
        <ProductPreview />
        <FeatureGrid />
        <HowItWorks />
        <Faq />
        <CtaSection />
      </main>
      <LandingFooter />
    </div>
  );
};

export default LandingHome;
