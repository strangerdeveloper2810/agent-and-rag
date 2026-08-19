import { useTranslation } from "react-i18next";
import { CheckIcon, MinusIcon } from "@heroicons/react/24/outline";

import { cn } from "@/lib/utils";
import { useRevealOnScroll } from "@/modules/landing/hooks/useRevealOnScroll";

const COMPARISON_ROWS = [
  { key: "rag", individual: true, team: true },
  { key: "multiAgent", individual: true, team: true },
  { key: "toolSkills", individual: true, team: true },
  { key: "documentUpload", individual: true, team: true },
  { key: "memory", individual: true, team: true },
  { key: "bilingual", individual: true, team: true },
  { key: "dataIsolation", individual: true, team: true },
  { key: "onboarding", individual: false, team: true },
] as const;

function ComparisonMark({ available }: { available: boolean }) {
  return available ? (
    <CheckIcon className="mx-auto h-4 w-4 text-primary" />
  ) : (
    <MinusIcon className="mx-auto h-4 w-4 text-muted-foreground/50" />
  );
}

/**
 * PricingComparison — bảng so sánh tính năng theo gói. Vì early-access chưa gate tính năng
 * theo gói, phần lớn hàng đều "có" ở cả 2 cột — điểm khác biệt duy nhất nằm ở mức hỗ trợ
 * onboarding (soft support commitment, không phải feature kỹ thuật bịa ra).
 */
export const PricingComparison: React.FC = () => {
  const { t } = useTranslation("landing");
  const { ref, isVisible } = useRevealOnScroll<HTMLDivElement>();

  return (
    <section className="border-y border-border bg-card/40 px-6 py-24 sm:py-28">
      <div className="mx-auto max-w-4xl">
        <div className="mx-auto max-w-2xl text-center">
          <span className="text-xs font-mono font-bold uppercase tracking-widest text-primary">
            {t("pricing.comparison.eyebrow")}
          </span>
          <h2 className="mt-3 font-display text-3xl font-extrabold tracking-tight text-foreground sm:text-4xl">
            {t("pricing.comparison.title")}
          </h2>
          <p className="mt-4 text-sm leading-relaxed text-muted-foreground sm:text-base">
            {t("pricing.comparison.subtitle")}
          </p>
        </div>

        <div
          ref={ref}
          className={cn(
            "mt-14 overflow-x-auto rounded-2xl border border-border bg-background",
            isVisible ? "animate-slide-up" : "opacity-0",
          )}
        >
          <table className="w-full min-w-[480px] text-left text-sm">
            <thead>
              <tr className="border-b border-border bg-muted/40">
                <th className="px-5 py-3 font-display text-xs font-bold uppercase tracking-wide text-muted-foreground">
                  {t("pricing.comparison.featureHeader")}
                </th>
                <th className="px-5 py-3 text-center font-display text-xs font-bold uppercase tracking-wide text-muted-foreground">
                  {t("pricing.comparison.planIndividual")}
                </th>
                <th className="px-5 py-3 text-center font-display text-xs font-bold uppercase tracking-wide text-muted-foreground">
                  {t("pricing.comparison.planTeam")}
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {COMPARISON_ROWS.map((row) => (
                <tr key={row.key}>
                  <td className="px-5 py-3.5 text-foreground">
                    {t(`pricing.comparison.rows.${row.key}`)}
                  </td>
                  <td className="px-5 py-3.5">
                    <ComparisonMark available={row.individual} />
                  </td>
                  <td className="px-5 py-3.5">
                    <ComparisonMark available={row.team} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </section>
  );
};

export default PricingComparison;
