import type { Config } from "tailwindcss";

// Color tokens map to CSS variables defined in /app/globals.css under
// :root (light) and .dark (dark). Adding `darkMode: 'class'` means
// any `dark:foo` utility activates when <html> carries the `dark` class.
const config: Config = {
  darkMode: "class",
  content: [
    "./app/**/*.{js,ts,jsx,tsx}",
    "./components/**/*.{js,ts,jsx,tsx}",
    "./lib/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        // Surfaces flip per theme (CSS vars).
        surface: {
          page: "var(--color-page)",
          card: "var(--color-card)",
          rail: "var(--color-rail)",
          line: "var(--color-line)",
        },
        ink: {
          DEFAULT: "var(--color-ink)",
          muted: "var(--color-ink-muted)",
          subtle: "var(--color-ink-subtle)",
        },
        // Status colors: 500-tier stays constant (used for icons, dots,
        // semi-transparent borders), 50 + 700 flip via CSS vars.
        ok: {
          50: "var(--color-ok-bg)",
          500: "#10b981",
          700: "var(--color-ok-text)",
        },
        warn: {
          50: "var(--color-warn-bg)",
          500: "#f59e0b",
          700: "var(--color-warn-text)",
        },
        block: {
          50: "var(--color-block-bg)",
          500: "#ef4444",
          700: "var(--color-block-text)",
        },
        brand: {
          50: "var(--color-brand-bg)",
          100: "#dbeafe",
          500: "#3b82f6",
          600: "#2563eb",
          700: "#1d4ed8",
        },
        // Severity palette retained (mostly used through severity-specific
        // tokens above).
        sev: {
          info: "#64748b",
          low: "#0ea5e9",
          medium: "#f59e0b",
          high: "#f97316",
          critical: "#ef4444",
        },
      },
      fontFamily: {
        sans: ["Inter", "ui-sans-serif", "system-ui", "sans-serif"],
        mono: ["JetBrains Mono", "ui-monospace", "monospace"],
      },
      boxShadow: {
        card: "var(--shadow-card)",
      },
    },
  },
  plugins: [],
};

export default config;
