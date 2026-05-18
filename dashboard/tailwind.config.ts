import type { Config } from "tailwindcss";

const config: Config = {
  content: [
    "./app/**/*.{js,ts,jsx,tsx}",
    "./components/**/*.{js,ts,jsx,tsx}",
    "./lib/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        // Severity palette — kept in one place so badges and row borders
        // stay in sync.
        sev: {
          info: "#94a3b8",      // slate-400
          low: "#3b82f6",       // blue-500
          medium: "#f59e0b",    // amber-500
          high: "#f97316",      // orange-500
          critical: "#ef4444",  // red-500
        },
      },
      fontFamily: {
        sans: ["Inter", "ui-sans-serif", "system-ui", "sans-serif"],
        mono: ["JetBrains Mono", "ui-monospace", "monospace"],
      },
    },
  },
  plugins: [],
};

export default config;
