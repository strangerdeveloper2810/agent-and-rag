import { useTranslation } from "react-i18next";
import {
  MagnifyingGlassIcon,
  CpuChipIcon,
  BoltIcon,
  CircleStackIcon,
  LanguageIcon,
} from "@heroicons/react/24/outline";

import { useDocumentTitle } from "@/hooks/useDocumentTitle";
import { LandingHeader } from "./components/LandingHeader";
import { FeatureDetail } from "./components/FeatureDetail";
import { CtaSection } from "./components/CtaSection";
import { LandingFooter } from "./components/LandingFooter";

const FEATURE_DETAIL_ITEMS = [
  { key: "rag", icon: MagnifyingGlassIcon },
  { key: "multiAgent", icon: CpuChipIcon },
  { key: "toolSkills", icon: BoltIcon },
  { key: "memory", icon: CircleStackIcon },
  { key: "bilingual", icon: LanguageIcon },
] as const;

/**
 * LandingFeatures — trang "/features": mở rộng chi tiết hơn 3 trụ cột đã tóm tắt ở Home
 * (RAG, multi-agent routing, tool skills — xem FeatureGrid.tsx) + 2 nhóm bổ sung có thật
 * trong hệ thống: bộ nhớ nhiều tầng (memory) và hỗ trợ song ngữ vi/en. KHÔNG thêm tính năng
 * không tồn tại. Xem docs/plans/2026-08-19-landing-page-ssr-design.md.
 */
export const LandingFeatures: React.FC = () => {
  const { t } = useTranslation("landing");
  useDocumentTitle(
    t("featuresPage.pageTitle"),
    t("featuresPage.pageDescription"),
  );

  return (
    <div className="h-screen overflow-y-auto scroll-fine bg-background text-foreground">
      <LandingHeader />
      <main>
        <section className="relative overflow-hidden px-6 pb-4 pt-16 sm:pt-24">
          <div className="pointer-events-none absolute -right-24 top-10 h-[420px] w-[420px] rounded-full bg-primary/10 blur-[130px]" />

          <div className="relative z-10 mx-auto max-w-3xl text-center">
            <span className="animate-fade-in text-xs font-mono font-bold uppercase tracking-widest text-primary">
              {t("featuresPage.hero.eyebrow")}
            </span>
            <h1 className="animate-slide-up mt-3 font-display text-4xl font-extrabold leading-tight tracking-tight text-foreground sm:text-5xl">
              {t("featuresPage.hero.title")}
            </h1>
            <p className="animate-slide-up mt-5 text-sm leading-relaxed text-muted-foreground sm:text-base">
              {t("featuresPage.hero.subtitle")}
            </p>
          </div>
        </section>

        {FEATURE_DETAIL_ITEMS.map(({ key, icon: Icon }, idx) => (
          <FeatureDetail
            key={key}
            itemKey={key}
            icon={Icon}
            reversed={idx % 2 === 1}
            tinted={idx % 2 === 1}
          />
        ))}

        <CtaSection />
      </main>
      <LandingFooter />
    </div>
  );
};

export default LandingFeatures;
