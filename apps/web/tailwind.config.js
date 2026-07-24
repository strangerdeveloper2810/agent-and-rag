import typography from "@tailwindcss/typography";

/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  darkMode: "class",
  theme: {
    extend: {
      fontFamily: {
        sans: [
          '"Roboto"',
          "ui-sans-serif",
          "system-ui",
          "Arial",
          "sans-serif",
        ],
      },
      colors: {
        // Colors driven by CSS variables (set in index.css :root and .dark)
        surface: "var(--color-surface)",
        subtle: "var(--color-subtle)",
        subtle2: "var(--color-subtle2)",
        ink: {
          DEFAULT: "var(--color-ink)",
          soft: "var(--color-ink-soft)",
          faint: "var(--color-ink-faint)",
        },
        line: "var(--color-line)",
        gblue: {
          DEFAULT: "var(--color-gblue)",
          bright: "var(--color-gblue-bright)",
          soft: "var(--color-gblue-soft)",
        },
      },
      backgroundImage: {
        gemini:
          "linear-gradient(74deg, #4285f4 0%, #9b72cb 47%, #d96570 100%)",
      },
      boxShadow: {
        soft: "0 1px 3px var(--color-shadow), 0 4px 12px -6px var(--color-shadow)",
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
        shimmer: {
          "0%": { backgroundPosition: "200% 0" },
          "100%": { backgroundPosition: "-200% 0" },
        },
        "spin-slow": {
          to: { transform: "rotate(360deg)" },
        },
      },
      animation: {
        "msg-in": "msg-in 0.4s cubic-bezier(0.22, 0.8, 0.36, 1) both",
        "fade-in": "fade-in 0.4s ease both",
        "dot-bounce": "dot-bounce 1.2s ease-in-out infinite",
        "caret-blink": "caret-blink 1s step-end infinite",
        shimmer: "shimmer 1.8s linear infinite",
        "spin-slow": "spin-slow 2s linear infinite",
      },
    },
  },
  plugins: [typography],
};
