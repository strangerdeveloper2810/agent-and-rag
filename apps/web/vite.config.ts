import { defineConfig, type Plugin } from "vite";
import { fileURLToPath, URL } from "node:url";
import react, { reactCompilerPreset } from "@vitejs/plugin-react";
import babel from "@rolldown/plugin-babel";

// Dev server có 2 "app" độc lập chia sẻ 1 Vite server: landing (index.html →
// entry-landing-client.tsx) và app CSR (app/index.html → main.tsx). Vite dev
// mặc định chỉ fallback về 1 index.html gốc cho path không khớp file nào — nếu
// không có middleware này, "/app", "/login"... sẽ vô tình load bundle landing.
// Production không cần middleware này: nginx tự route theo location block
// (xem nginx.conf) sau khi build ra 2 thư mục dist/ và dist/app/ riêng.
function devMpaRoutingPlugin(): Plugin {
  const APP_PREFIXES = [
    "/app",
    "/login",
    "/register",
    "/verify-email",
    "/forgot-password",
  ];
  return {
    name: "dev-mpa-routing",
    configureServer(server) {
      server.middlewares.use((req, _res, next) => {
        const url = req.url ?? "";
        const pathname = url.split("?")[0];
        const isAppRoute = APP_PREFIXES.some(
          (p) => pathname === p || pathname.startsWith(`${p}/`),
        );
        // Chỉ rewrite request điều hướng trang (không có phần mở rộng file) —
        // asset thật (/assets/*.js, /favicon.svg...) đi qua bình thường.
        if (isAppRoute && !pathname.includes(".")) {
          req.url = "/app/index.html";
        }
        next();
      });
    },
  };
}

export default defineConfig({
  plugins: [
    react(),
    // React Compiler (GA): tự memo hoá component/hook lúc build → giảm re-render
    // thừa mà không cần useMemo/useCallback thủ công. plugin-react v6 dùng oxc nên
    // compiler chạy qua @rolldown/plugin-babel + preset (React 19 không cần runtime riêng).
    babel({ presets: [reactCompilerPreset()] }),
    devMpaRoutingPlugin(),
  ],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  server: {
    port: 3000,
    proxy: {
      "/api": "http://localhost:3001",
    },
  },
  build: {
    // Cảnh báo khi 1 chunk vượt ngưỡng (kB, sau minify — trước gzip).
    chunkSizeWarningLimit: 600,
    // Mặc định Vite chèn <link rel="modulepreload"> cho MỌI chunk mà 1 entry có
    // thể-với-tới, kể cả qua nhánh import() động — kể cả khi entry đó (landing
    // hay app) không thực sự dùng tới (markdown/mermaid chỉ ChatPage dùng, và
    // mermaid vốn đã cố tình tách dynamic import — xem MermaidBlock.tsx). Nếu
    // không lọc ra, browser tải trước ~830kB mermaid ngay khi vào TRANG CHỦ,
    // xoá sạch lợi ích của việc tách dynamic import. Chỉ preload phần thực sự
    // cần cho lần vẽ đầu tiên.
    modulePreload: {
      resolveDependencies: (_filename, deps) =>
        deps.filter((dep) => !/\/(markdown|mermaid)-[^/]+\.js$/.test(dep)),
    },
    rollupOptions: {
      // 2 HTML entry riêng biệt trong 1 lần build: index.html (landing, bundle
      // nhẹ, không QueryClient/Zustand/router) và app/index.html (CSR app đầy
      // đủ, main.tsx). Vite giữ nguyên cấu trúc thư mục tương đối khi build →
      // output đúng dist/index.html + dist/app/index.html.
      input: {
        landing: fileURLToPath(new URL("./index.html", import.meta.url)),
        app: fileURLToPath(new URL("./app/index.html", import.meta.url)),
      },
      output: {
        // Vite 8 dùng Rolldown → manualChunks chỉ nhận dạng HÀM (không nhận object).
        // Tách vendor nặng ra chunk riêng: markdown (chỉ dùng ở chat) + react core.
        // → cache tốt hơn (đổi code app không bust cache vendor) + tải song song.
        manualChunks(id) {
          if (!id.includes("/node_modules/")) return;
          if (
            id.includes("/react-markdown/") ||
            id.includes("/remark") ||
            id.includes("/micromark") ||
            id.includes("/mdast") ||
            id.includes("/hast") ||
            id.includes("/unist") ||
            id.includes("/unified") ||
            id.includes("/vfile")
          ) {
            return "markdown";
          }
          // mermaid được import động (xem MermaidBlock.tsx) nên vốn đã tách chunk
          // riêng theo cơ chế code-splitting mặc định của Rollup; rule này chỉ đảm
          // bảo mọi phần phụ thuộc của nó gộp cùng 1 chunk ổn định, dễ cache.
          if (id.includes("/mermaid/") || id.includes("/mermaid-")) {
            return "mermaid";
          }
          if (
            id.includes("/react-router") ||
            id.includes("/react-dom/") ||
            id.includes("/react/") ||
            id.includes("/scheduler/")
          ) {
            return "react-vendor";
          }
        },
      },
    },
  },
});
