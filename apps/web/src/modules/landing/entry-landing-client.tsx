import { hydrateRoot, createRoot } from "react-dom/client";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18next from "i18next";
import { resources, defaultNS } from "@/i18n/resources";
import { LANDING_PAGES, parseLandingPath } from "./pages";
import "@/index.css";

// Bundle NÀY hoàn toàn tách biệt với main.tsx (CSR app ở /app): không
// QueryClientProvider, không BrowserRouter/Zustand — chỉ i18n + component landing,
// giữ bundle nhỏ đúng như thiết kế đã chốt.
const { locale, page } = parseLandingPath(window.location.pathname);

const i18nInstance = i18next.createInstance();

void i18nInstance
  .use(initReactI18next)
  .init({
    lng: locale,
    fallbackLng: "vi",
    defaultNS,
    ns: Object.keys(resources.vi),
    resources,
    interpolation: { escapeValue: false },
    react: { useSuspense: false },
  })
  .then(() => {
    const Page = LANDING_PAGES[page];
    const root = document.getElementById("root");
    if (!root) return;
    const element = (
      <I18nextProvider i18n={i18nInstance}>
        <Page />
      </I18nextProvider>
    );
    // Dev server (Vite) KHÔNG chạy qua prerender.mjs — #root luôn rỗng, nên
    // hydrateRoot() sẽ báo mismatch (không có gì để "khớp"). Chỉ hydrate thật
    // khi #root đã có sẵn nội dung SSR (build production); dev mode mount CSR
    // bình thường bằng createRoot().
    if (import.meta.env.DEV || root.childElementCount === 0) {
      createRoot(root).render(element);
    } else {
      hydrateRoot(root, element);
    }
  });
