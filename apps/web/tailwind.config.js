import typography from "@tailwindcss/typography";

/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      fontFamily: {
        sans: ['"Be Vietnam Pro"', "ui-sans-serif", "system-ui", "sans-serif"],
        display: ['"Bricolage Grotesque"', "ui-sans-serif", "sans-serif"],
      },
      colors: {
        paper: "#F4F2EC",
        surface: "#FBFAF7",
        ink: { DEFAULT: "#21201C", soft: "#6B665C", faint: "#9A9486" },
        line: "#E7E3D9",
        accent: {
          DEFAULT: "#157F69",
          ink: "#0E5A4A",
          soft: "#E3F1EC",
          glow: "#1FA688",
        },
      },
      boxShadow: {
        soft: "0 1px 2px rgba(33,32,28,0.04), 0 8px 24px -12px rgba(33,32,28,0.14)",
        bubble:
          "0 1px 1px rgba(33,32,28,0.06), 0 8px 18px -10px rgba(21,127,105,0.45)",
        ring: "0 0 0 1px rgba(33,32,28,0.06)",
      },
      keyframes: {
        "msg-in": {
          from: { opacity: "0", transform: "translateY(10px)" },
          to: { opacity: "1", transform: "none" },
        },
        "fade-in": {
          from: { opacity: "0" },
          to: { opacity: "1" },
        },
        "dot-bounce": {
          "0%, 80%, 100%": { transform: "translateY(0)", opacity: "0.4" },
          "40%": { transform: "translateY(-4px)", opacity: "1" },
        },
        "caret-blink": {
          "0%, 100%": { opacity: "1" },
          "50%": { opacity: "0" },
        },
      },
      animation: {
        "msg-in": "msg-in 0.45s cubic-bezier(0.22, 0.8, 0.36, 1) both",
        "fade-in": "fade-in 0.4s ease both",
        "dot-bounce": "dot-bounce 1.2s ease-in-out infinite",
        "caret-blink": "caret-blink 1s step-end infinite",
      },
    },
  },
  plugins: [typography],
};
