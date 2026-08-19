import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";

export type LandingSlug = "" | "pricing" | "features";

/**
 * LandingLanguageSwitcher — KHÁC atom LanguageSwitcher dùng ở app CSR (đổi ngôn ngữ
 * tại chỗ qua i18n.changeLanguage). Mỗi trang landing được prerender thành 1 file HTML
 * RIÊNG cho từng locale ("/pricing" = vi, "/en/pricing" = en) — đổi ngôn ngữ ở đây
 * PHẢI là điều hướng sang URL khác (full page load), không phải đổi state tại chỗ.
 */
export const LandingLanguageSwitcher: React.FC<{ slug: LandingSlug }> = ({
  slug,
}) => {
  const { i18n } = useTranslation();
  const locale = i18n.language?.startsWith("en") ? "en" : "vi";
  const path = slug ? `/${slug}` : "";
  const viHref = path || "/";
  const enHref = `/en${path}`;

  return (
    <div
      role="group"
      aria-label="Language selection"
      className="inline-flex items-center rounded-full border border-border/50 bg-muted/60 p-0.5 text-[11px] font-medium tracking-wide shadow-xs backdrop-blur-sm"
    >
      <a
        href={viHref}
        aria-current={locale === "vi" ? "page" : undefined}
        className={cn(
          "rounded-full px-2.5 py-1 transition-all duration-200",
          locale === "vi"
            ? "border border-border/40 bg-background font-bold text-foreground shadow-xs"
            : "text-muted-foreground hover:bg-background/40 hover:text-foreground",
        )}
      >
        VI
      </a>
      <a
        href={enHref}
        aria-current={locale === "en" ? "page" : undefined}
        className={cn(
          "rounded-full px-2.5 py-1 transition-all duration-200",
          locale === "en"
            ? "border border-border/40 bg-background font-bold text-foreground shadow-xs"
            : "text-muted-foreground hover:bg-background/40 hover:text-foreground",
        )}
      >
        EN
      </a>
    </div>
  );
};

export default LandingLanguageSwitcher;
