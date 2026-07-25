import typography from "@tailwindcss/typography";

/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  darkMode: "class",
  theme: {
    extend: {
      fontFamily: {
        display: ['"Outfit"', "ui-sans-serif", "system-ui", "sans-serif"],
        sans: [
          '"Plus Jakarta Sans"',
          "ui-sans-serif",
          "system-ui",
          "sans-serif",
        ],
        mono: [
          '"JetBrains Mono"',
          "ui-monospace",
          "SF Mono",
          "Consolas",
          "monospace",
        ],
      },
      colors: {
        // Cyberpunk palette (CSS vars set in index.css)
        cyber: {
          bg: "var(--cyber-bg)",
          surface: "var(--cyber-surface)",
          primary: "var(--cyber-primary)",
          secondary: "var(--cyber-secondary)",
          accent: "var(--cyber-accent)",
          text: "var(--cyber-text)",
          muted: "var(--cyber-muted)",
          border: "var(--cyber-border)",
          success: "var(--cyber-success)",
          error: "var(--cyber-error)",
          glow: "var(--cyber-glow)",
        },
        // Keep legacy aliases so existing utility references still work
        surface: "var(--cyber-surface)",
        subtle: "var(--cyber-subtle)",
        subtle2: "var(--cyber-subtle2)",
        ink: {
          DEFAULT: "var(--cyber-text)",
          soft: "var(--cyber-muted)",
          faint: "var(--cyber-faint)",
        },
        line: "var(--cyber-border)",
        gblue: {
          DEFAULT: "var(--cyber-primary)",
          bright: "var(--cyber-primary)",
          soft: "var(--cyber-primary-soft)",
        },
      },
      boxShadow: {
        soft: "0 1px 3px rgba(0,0,0,0.4), 0 4px 12px -6px rgba(0,0,0,0.5)",
        "neon-cyan":
          "0 0 5px var(--cyber-primary), inset 0 0 5px var(--cyber-primary)",
        "neon-magenta":
          "0 0 5px var(--cyber-secondary), inset 0 0 5px var(--cyber-secondary)",
        "neon-glow": "0 0 15px var(--cyber-glow)",
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
        "terminal-blink": {
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
        "neon-pulse": {
          "0%, 100%": {
            textShadow:
              "0 0 7px var(--cyber-primary), 0 0 20px var(--cyber-primary)",
          },
          "50%": {
            textShadow:
              "0 0 4px var(--cyber-primary), 0 0 10px var(--cyber-primary)",
          },
        },
        "scanline-drift": {
          "0%": { transform: "translateY(0)" },
          "100%": { transform: "translateY(4px)" },
        },
        glitch: {
          "0%, 100%": { transform: "translate(0)" },
          "20%": { transform: "translate(-1px, 1px)" },
          "40%": { transform: "translate(1px, -1px)" },
          "60%": { transform: "translate(-1px, -1px)" },
          "80%": { transform: "translate(1px, 1px)" },
        },
        stream: {
          from: { opacity: "0", transform: "translateY(4px)" },
          to: { opacity: "1", transform: "translateY(0)" },
        },
        "slide-up": {
          from: { opacity: "0", transform: "translateY(12px)" },
          to: { opacity: "1", transform: "none" },
        },
      },
      animation: {
        "msg-in": "msg-in 0.3s cubic-bezier(0.22, 0.8, 0.36, 1) both",
        "fade-in": "fade-in 0.3s ease both",
        "dot-bounce": "dot-bounce 1.2s ease-in-out infinite",
        "caret-blink": "caret-blink 1s step-end infinite",
        "terminal-blink": "terminal-blink 1s step-end infinite",
        shimmer: "shimmer 1.8s linear infinite",
        "spin-slow": "spin-slow 2s linear infinite",
        "neon-pulse": "neon-pulse 2s ease-in-out infinite",
        "scanline-drift": "scanline-drift 8s linear infinite",
        glitch: "glitch 0.3s ease-in-out infinite",
        stream: "stream 0.2s ease-out",
        "slide-up": "slide-up 0.3s ease-out both",
      },
    },
  },
  plugins: [typography],
};
