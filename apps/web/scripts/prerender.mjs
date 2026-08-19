// Prerender (SSG) cho landing page: gọi render() từ bundle SSR đã build
// (dist-ssr/entry-landing-server.js) cho từng (page × locale), chèn HTML +
// SEO tags vào template dist/index.html (đã build bởi bước `vite build` client
// ở trên), ghi ra file tĩnh. Sinh kèm sitemap.xml + robots.txt.
//
// Chạy SAU KHI đã có: `vite build` (dist/index.html + dist/app/index.html +
// dist/assets/*) và `vite build --ssr src/modules/landing/entry-landing-server.tsx
// --outDir dist-ssr`. Xem package.json script "build".

import fs from "node:fs";
import path from "node:path";
import { pathToFileURL } from "node:url";

const DIST_DIR = path.resolve("dist");
const SSR_ENTRY = path.resolve("dist-ssr/entry-landing-server.js");
const SITE_URL = "https://ai.ethansoftwaredeveloper.com";

const PAGES = [
  { page: "home", slug: "" },
  { page: "pricing", slug: "pricing" },
  { page: "features", slug: "features" },
];
const LOCALES = ["vi", "en"];

/** URL path public (không gồm domain) cho 1 (slug, locale). */
function urlPath(slug, locale) {
  const base = slug ? `/${slug}` : "/";
  if (locale === "vi") return base;
  return slug ? `/en/${slug}` : "/en";
}

/** Đường dẫn file output tương ứng trong dist/. */
function outputFile(slug, locale) {
  const dir = locale === "en" ? path.join(DIST_DIR, "en") : DIST_DIR;
  const targetDir = slug ? path.join(dir, slug) : dir;
  return path.join(targetDir, "index.html");
}

function escapeHtml(value) {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

function buildHeadTags({ page, slug, locale, title, description }) {
  const canonicalPath = urlPath(slug, locale);
  const canonicalUrl = `${SITE_URL}${canonicalPath}`;
  const viUrl = `${SITE_URL}${urlPath(slug, "vi")}`;
  const enUrl = `${SITE_URL}${urlPath(slug, "en")}`;
  const fullTitle = page === "home" ? title : `${title} — J.A.R.V.I.S.`;
  const ogLocale = locale === "en" ? "en_US" : "vi_VN";
  const ogLocaleAlt = locale === "en" ? "vi_VN" : "en_US";

  const jsonLd = {
    "@context": "https://schema.org",
    "@graph": [
      {
        "@type": "SoftwareApplication",
        "@id": `${SITE_URL}/#application`,
        name: "J.A.R.V.I.S.",
        url: SITE_URL,
        description,
        applicationCategory:
          "BusinessApplication, DeveloperApplication, ArtificialIntelligence",
        operatingSystem: "Web, All",
        featureList: [
          "Multi-Agent AI Routing (DeepSeek, Gemini, Claude)",
          "Semantic RAG Vector Search",
          "Real-time Streaming SSE",
          "30+ Autonomous Tool Skills",
        ],
        publisher: {
          "@type": "Organization",
          name: "Ethan Software",
          url: "https://ethansoftwaredeveloper.com",
        },
      },
      {
        "@type": "WebSite",
        "@id": `${SITE_URL}/#website`,
        url: SITE_URL,
        name: "J.A.R.V.I.S. AI Platform",
        description,
        publisher: { "@type": "Organization", name: "Ethan Software" },
        inLanguage: locale === "en" ? "en-US" : "vi-VN",
      },
    ],
  };

  return `
    <title>${escapeHtml(fullTitle)}</title>
    <meta name="description" content="${escapeHtml(description)}" />
    <link rel="canonical" href="${canonicalUrl}" />
    <link rel="alternate" hreflang="vi" href="${viUrl}" />
    <link rel="alternate" hreflang="en" href="${enUrl}" />
    <link rel="alternate" hreflang="x-default" href="${viUrl}" />
    <meta property="og:type" content="website" />
    <meta property="og:site_name" content="J.A.R.V.I.S. AI Platform" />
    <meta property="og:url" content="${canonicalUrl}" />
    <meta property="og:title" content="${escapeHtml(fullTitle)}" />
    <meta property="og:description" content="${escapeHtml(description)}" />
    <meta property="og:image" content="${SITE_URL}/og-image.svg" />
    <meta property="og:locale" content="${ogLocale}" />
    <meta property="og:locale:alternate" content="${ogLocaleAlt}" />
    <meta name="twitter:card" content="summary_large_image" />
    <meta name="twitter:title" content="${escapeHtml(fullTitle)}" />
    <meta name="twitter:description" content="${escapeHtml(description)}" />
    <meta name="twitter:image" content="${SITE_URL}/og-image.svg" />
    <script type="application/ld+json">${JSON.stringify(jsonLd)}</script>`;
}

function generateSitemap(entries) {
  const urls = entries
    .map(
      ({ slug, locale }) => `  <url>
    <loc>${SITE_URL}${urlPath(slug, locale)}</loc>
    <changefreq>weekly</changefreq>
    <priority>${slug ? "0.8" : "1.0"}</priority>
  </url>`,
    )
    .join("\n");
  return `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
${urls}
</urlset>
`;
}

function generateRobotsTxt() {
  return `User-agent: *
Allow: /
Disallow: /api/
Disallow: /app
Disallow: /verify-email

Sitemap: ${SITE_URL}/sitemap.xml
`;
}

async function main() {
  const templatePath = path.join(DIST_DIR, "index.html");
  if (!fs.existsSync(templatePath)) {
    throw new Error(
      `Không tìm thấy ${templatePath} — chạy "vite build" trước khi prerender.`,
    );
  }
  if (!fs.existsSync(SSR_ENTRY)) {
    throw new Error(
      `Không tìm thấy ${SSR_ENTRY} — chạy "vite build --ssr src/modules/landing/entry-landing-server.tsx --outDir dist-ssr" trước.`,
    );
  }

  const template = fs.readFileSync(templatePath, "utf-8");
  const { render } = await import(pathToFileURL(SSR_ENTRY).href);

  const entries = [];
  for (const { page, slug } of PAGES) {
    for (const locale of LOCALES) {
      entries.push({ page, slug, locale });
    }
  }

  for (const { page, slug, locale } of entries) {
    const { html, title, description } = await render(page, locale);
    const headTags = buildHeadTags({ page, slug, locale, title, description });

    let output = template
      .replace(/<html lang="vi"/, `<html lang="${locale}"`)
      .replace("</head>", `${headTags}\n  </head>`)
      .replace('<div id="root"></div>', `<div id="root">${html}</div>`);

    const outFile = outputFile(slug, locale);
    fs.mkdirSync(path.dirname(outFile), { recursive: true });
    fs.writeFileSync(outFile, output);
    console.log(
      `[prerender] ${page}/${locale} -> ${path.relative(DIST_DIR, outFile)}`,
    );
  }

  fs.writeFileSync(
    path.join(DIST_DIR, "sitemap.xml"),
    generateSitemap(entries),
  );
  fs.writeFileSync(path.join(DIST_DIR, "robots.txt"), generateRobotsTxt());
  console.log("[prerender] sitemap.xml + robots.txt generated");
}

main().catch((err) => {
  console.error("[prerender] failed:", err);
  process.exit(1);
});
