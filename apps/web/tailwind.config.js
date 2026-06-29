import typography from "@tailwindcss/typography";

/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      fontFamily: {
        sans: ['"Roboto"', "ui-sans-serif", "system-ui", "Arial", "sans-serif"],
      },
      colors: {
        // Bảng màu kiểu Google / Gemini
        surface: "#ffffff",
        // nền phụ xanh-xám nhạt (sidebar, input, bubble user) — đặc trưng Gemini
        subtle: "#f0f4f9",
        subtle2: "#e9eef6",
        ink: { DEFAULT: "#1f1f1f", soft: "#444746", faint: "#5e5e5e" },
        line: "#dde3ea",
        // Google Blue
        gblue: { DEFAULT: "#0b57d0", bright: "#1a73e8", soft: "#d3e3fd" },
      },
      backgroundImage: {
        // Dải gradient sao Gemini (xanh → tím → hồng)
        gemini: "linear-gradient(74deg, #4285f4 0%, #9b72cb 47%, #d96570 100%)",
      },
      boxShadow: {
        soft: "0 1px 3px rgba(0,0,0,0.08), 0 4px 12px -6px rgba(0,0,0,0.12)",
        ring: "0 0 0 1px rgba(0,0,0,0.06)",
      },
      keyframes: {
        "msg-in": {
          from: { opacity: "0", transform: "translateY(8px)" },
          to: { opacity: "1", transform: "none" },
        },
        "fade-in": { from: { opacity: "0" }, to: { opacity: "1" } },
        "dot-bounce": {
          "0%, 80%, 100%": { transform: "translateY(0)", opacity: "0.4" },
          "40%": { transform: "translateY(-4px)", opacity: "1" },
        },
        "caret-blink": {
          "0%, 100%": { opacity: "1" },
          "50%": { opacity: "0" },
        },
        // Quét sáng chạy ngang chữ — hiệu ứng "đang xử lý" mềm mại
        shimmer: {
          "0%": { backgroundPosition: "200% 0" },
          "100%": { backgroundPosition: "-200% 0" },
        },
      },
      animation: {
        "msg-in": "msg-in 0.4s cubic-bezier(0.22, 0.8, 0.36, 1) both",
        "fade-in": "fade-in 0.4s ease both",
        "dot-bounce": "dot-bounce 1.2s ease-in-out infinite",
        "caret-blink": "caret-blink 1s step-end infinite",
        shimmer: "shimmer 1.8s linear infinite",
      },
    },
  },
  plugins: [typography],
};
