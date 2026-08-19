# Landing Page (Vite SSG/SSR) cho J.A.R.V.I.S. — Design

Date: 2026-08-19
Status: Approved (design), pending implementation plan

## Mục tiêu

Xây landing page public cho agent J.A.R.V.I.S., tận dụng khả năng SSR của Vite, đồng thời:
1. Tối ưu SEO (per-page meta/OG/JSON-LD, hreflang vi/en, sitemap/robots tự sinh).
2. Tối ưu bundle/chunk size cho cả landing lẫn app chat hiện tại.

## Khảo sát hiện trạng (tóm tắt)

- `apps/web` là SPA thuần CSR (Vite + React Router). Route `"/"` hiện là `ChatPage` bọc trong `AuthGuard` — **chưa có landing page public nào**, khách vãng lai vào domain bị redirect thẳng sang `/login`.
- `index.html` đã có sẵn SEO/OG/Twitter/JSON-LD khá đầy đủ nhưng hard-code cứng cho đúng 1 "trang" (không phân biệt route/locale).
- i18n (`react-i18next`, locale `vi`/`en`) đã merge vào `master` (`apps/web/src/i18n/`, `LanguageSwitcher`), nhưng `LanguageSwitcher` mới chỉ có trong `Header` (dùng ở `AppLayout` sau khi login) — **chưa có ở landing/trang public**.
- `MermaidBlock.tsx` (`design-system/molecules/`) import `mermaid` **tĩnh** — thư viện nặng bị gộp vào chunk của `ChatPage` dù phần lớn hội thoại không dùng diagram.
- `vite.config.ts` đã có `manualChunks` tách `react-vendor` + `markdown` (react-markdown/remark/...), nhưng chưa tách `mermaid`.
- `robots.txt`/`sitemap.xml` (`apps/web/public/`) là file tĩnh tay-maintain, hiện `Allow: /` (đang vô tình cho crawl cả trang bị AuthGuard-redirect), sitemap liệt kê cả `/login`, `/register` (không cần SEO).
- Deploy hiện tại: `Dockerfile` build client bằng `vite build` → copy `dist/` vào `nginx:1.27-alpine`, `nginx.conf` proxy `/api` → BFF Fastify, `/suggestions` → agent-go, cache `/assets/` 1 năm, gzip bật sẵn (không có brotli).

## Kiến trúc routing

Tách 2 "thế giới" trong cùng 1 app, build ra chung `dist/`, runtime deploy không đổi (vẫn nginx static + proxy):

```
"/"                                  → Landing home (SSG, public, vi)
"/pricing"                           → Landing pricing (SSG, public, vi)
"/features"                          → Landing features (SSG, public, vi)
"/en", "/en/pricing", "/en/features" → bản tiếng Anh tương ứng (SSG)
"/app"                               → ChatPage (AuthGuard, CSR) — trang chủ sau login (thay cho "/" cũ)
"/app/messages/:id"                  → ChatPage với id
"/app/documents"                     → DocumentsView
"/login", "/register",
"/verify-email", "/forgot-password"  → giữ nguyên (GuestGuard); sau login redirect sang "/app"
```

Landing và App dùng **2 bundle JS tách biệt**:
- Landing: entry riêng, chỉ `I18nextProvider` — không kéo theo `QueryClientProvider`/Zustand/`react-router` toàn app. Nội dung đã "đóng băng" theo locale lúc prerender (đổi ngôn ngữ = điều hướng sang route `/en/...`, không cần đổi runtime), nên phần cần hydrate rất nhỏ (mobile nav toggle, FAQ accordion nếu có).
- App: giữ nguyên `main.tsx`/`App.tsx` hiện tại (CSR đầy đủ), chỉ đổi route gốc từ `/` sang `/app`.

## Build pipeline: Vite SSR + prerender (SSG, không chạy Node server ở runtime)

Theo pattern chuẩn của Vite SSR guide — build client + server rồi render tĩnh 1 lần lúc build:

```
apps/web/src/
  entry-landing-client.tsx   # hydrateRoot(root, <LandingApp/>)
  entry-landing-server.tsx   # export render(url, locale) → renderToString(<LandingApp .../>)
  modules/landing/
    LandingHome.tsx / LandingPricing.tsx / LandingFeatures.tsx
    LandingHeader.tsx (có LanguageSwitcher) / LandingFooter.tsx
  main.tsx                    # giữ nguyên — entry cho "/app" (CSR), không đổi

scripts/prerender.mjs         # script Node mới, chạy lúc build
```

Build steps (`apps/web/package.json`):
1. `vite build` — build app CSR hiện tại → `dist/app/`
2. `vite build --outDir dist/landing-client --ssrManifest` — build client bundle riêng cho landing
3. `vite build --ssr src/entry-landing-server.tsx --outDir dist-ssr` — build bundle server để render
4. `node scripts/prerender.mjs` — loop qua danh sách `{path, locale}` (6 route: `/`, `/pricing`, `/features` × vi/en), gọi `render()` từ bundle bước 3, chèn HTML + head-tags (mục SEO) vào template, ghi `dist/index.html`, `dist/pricing/index.html`, `dist/en/index.html`, ...

Runtime **không đổi**: nginx chỉ serve static `dist/` + proxy `/api`, `/suggestions`. `Dockerfile` chỉ thêm các bước build trên ở stage `build`, stage runtime vẫn `nginx:1.27-alpine`.

`nginx.conf` cần sửa 1 chỗ: thêm `location /app/ { try_files $uri $uri/ /app/index.html; }` (SPA fallback chỉ áp dụng cho `/app/*`); `location /` giữ nguyên serve file tĩnh thật (landing đã là HTML tĩnh, không cần fallback).

## SEO

- `index.html` hiện tại đổi thành **template** với placeholder (`<!--head-tags-->`), bỏ nội dung SEO hard-code cho đúng 1 trang.
- `scripts/prerender.mjs` tự sinh cho từng route × locale: title/description/canonical, OG/Twitter, JSON-LD phù hợp nội dung trang (Home: `SoftwareApplication`+`WebSite`; Pricing: thêm `Offer`; Features: có thể `FAQPage` nếu có mục FAQ).
- hreflang: mỗi trang tự link `<link rel="alternate" hreflang="vi|en|x-default">` sang bản dịch còn lại — tránh Google coi vi/en là duplicate content.
- `robots.txt`: thêm `Disallow: /app` (chat sau AuthGuard, không cần index). Bỏ `/login`, `/register` khỏi `sitemap.xml`.
- `sitemap.xml`: sinh **tự động** trong bước prerender từ cùng danh sách route, không tay-maintain nữa.
- Core Web Vitals: SSG cho LCP tốt sẵn (HTML có nội dung ngay). Bổ sung: preload hero image/font quan trọng theo từng trang, ảnh hero khai báo đúng width/height (tránh CLS), cân nhắc self-host font JetBrains Mono/Roboto thay vì gọi Google Fonts.

## Bundle/chunk optimization

**Landing**: bundle tối giản theo kiến trúc trên (không QueryClient/Zustand/router) → mục tiêu vài chục KB gzip.

**App (`/app`)**:
- `MermaidBlock.tsx`: đổi `import mermaid from "mermaid"` (tĩnh) → `const mermaid = await import("mermaid")` (động, chỉ tải khi thực sự gặp block ```mermaid).
- `vite.config.ts` `manualChunks`: thêm rule tách riêng `mermaid` (tương tự cách đang tách `markdown`) để cache độc lập.
- `nginx.conf`: giữ cache `/assets/` 1 năm (đúng vì filename có hash); thêm rule KHÔNG cache dài file HTML landing (nội dung có thể đổi, cần revalidate).
- Brotli: **bỏ qua giai đoạn này** — `nginx:alpine` không có module brotli sẵn (phải build lại image), ROI thấp hơn code-splitting; gzip hiện tại là đủ.

## Nội dung landing (phác thảo, sẽ viết copy chi tiết lúc implement)

- **Home** (`/`): Hero (tên + tagline + CTA) → Features tóm tắt (RAG, multi-agent routing, 30+ skills) → How it works → CTA cuối (Đăng ký/Đăng nhập).
- **Pricing** (`/pricing`): Bảng giá/gói (nếu có), so sánh tính năng.
- **Features** (`/features`): Chi tiết từng nhóm tính năng (RAG semantic search, multi-agent, memory, tool skills).

## Testing

- Component test cho `modules/landing/*` bằng Vitest + Testing Library (giống pattern hiện tại của `apps/web`).
- Test riêng cho `scripts/prerender.mjs`: chạy thử build + kiểm tra `dist/index.html`, `dist/en/pricing/index.html`... tồn tại, có đúng thẻ `<title>`/`hreflang`/JSON-LD mong đợi.
- Kiểm tra thủ công: `pnpm --filter @app/web build && pnpm --filter @app/web preview` (hoặc serve `dist/` qua nginx local) — xác nhận landing hiển thị đúng không cần JS (view-source), `/app` vẫn hoạt động như SPA cũ, redirect sau login trỏ đúng `/app`.

## Ngoài phạm vi (follow-up sau)

- Kiến trúc "island" hydrate từng phần nhỏ riêng lẻ thay vì cả cây landing (chỉ cần nếu landing phình to sau này).
- Brotli compression.
- Trang `/blog`.
