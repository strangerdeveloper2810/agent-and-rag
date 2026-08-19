import LandingHome from "./LandingHome";
import LandingPricing from "./LandingPricing";
import LandingFeatures from "./LandingFeatures";

export const LANDING_PAGES = {
  home: LandingHome,
  pricing: LandingPricing,
  features: LandingFeatures,
} as const;

export type LandingPageKey = keyof typeof LANDING_PAGES;
export type Locale = "vi" | "en";

/**
 * Suy ra locale + page key từ 1 pathname thật — dùng chung giữa prerender script
 * (server, biết trước path đang render) và entry-landing-client.tsx (client, đọc
 * window.location.pathname lúc hydrate). Phải khớp 100% giữa 2 bên để tránh
 * hydration mismatch.
 */
export function parseLandingPath(pathname: string): {
  locale: Locale;
  page: LandingPageKey;
} {
  let path = pathname;
  let locale: Locale = "vi";
  if (path === "/en" || path.startsWith("/en/")) {
    locale = "en";
    path = path.slice(3) || "/";
  }
  // Chuẩn hoá trailing slash (trừ root "/") — một số static server/proxy trả về
  // URL dạng "/pricing/", nếu không chuẩn hoá sẽ rơi vào "home" và gây
  // hydration mismatch (server render Pricing, client hydrate nhầm Home).
  if (path.length > 1 && path.endsWith("/")) {
    path = path.slice(0, -1);
  }
  const page: LandingPageKey =
    path === "/pricing"
      ? "pricing"
      : path === "/features"
        ? "features"
        : "home";
  return { locale, page };
}
