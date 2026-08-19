import { defineConfig } from "vite";
import { fileURLToPath, URL } from "node:url";
import react, { reactCompilerPreset } from "@vitejs/plugin-react";
import babel from "@rolldown/plugin-babel";

export default defineConfig({
  plugins: [
    react(),
    // React Compiler (GA): tự memo hoá component/hook lúc build → giảm re-render
    // thừa mà không cần useMemo/useCallback thủ công. plugin-react v6 dùng oxc nên
    // compiler chạy qua @rolldown/plugin-babel + preset (React 19 không cần runtime riêng).
    babel({ presets: [reactCompilerPreset()] }),
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
    rollupOptions: {
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
