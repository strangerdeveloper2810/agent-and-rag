import { renderToString } from "react-dom/server";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18next from "i18next";
import { resources, defaultNS } from "@/i18n/resources";
import { LANDING_PAGES, type LandingPageKey, type Locale } from "./pages";

// key i18n (namespace "landing") chứa title/description cho từng trang — nguồn
// duy nhất cho cả UI lẫn thẻ SEO, tránh trùng lặp copy giữa 2 nơi.
const META_KEYS: Record<LandingPageKey, { title: string; description: string }> = {
  home: { title: "pageTitle", description: "pageDescription" },
  pricing: { title: "pricing.pageTitle", description: "pricing.pageDescription" },
  features: {
    title: "featuresPage.pageTitle",
    description: "featuresPage.pageDescription",
  },
};

export interface LandingRenderResult {
  html: string;
  title: string;
  description: string;
}

/**
 * render() — gọi 1 lần cho mỗi (page, locale) lúc build (prerender.mjs), KHÔNG
 * chạy per-request (đây là SSG, không phải SSR server sống). Tạo i18next
 * instance MỚI mỗi lần gọi (không dùng singleton "@/i18n" — instance đó gọi
 * getInitialLocale() phụ thuộc localStorage/window, crash trên Node).
 */
export async function render(
  page: LandingPageKey,
  locale: Locale,
): Promise<LandingRenderResult> {
  const i18nInstance = i18next.createInstance();
  await i18nInstance.use(initReactI18next).init({
    lng: locale,
    fallbackLng: "vi",
    defaultNS,
    ns: Object.keys(resources.vi),
    resources,
    interpolation: { escapeValue: false },
    react: { useSuspense: false },
  });

  const Page = LANDING_PAGES[page];
  const html = renderToString(
    <I18nextProvider i18n={i18nInstance}>
      <Page />
    </I18nextProvider>,
  );

  const t = i18nInstance.getFixedT(locale, "landing");
  const { title, description } = META_KEYS[page];
  return { html, title: t(title), description: t(description) };
}
