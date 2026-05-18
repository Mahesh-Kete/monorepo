"use client";

import { useEffect, useState } from "react";
import { Sun, Moon } from "lucide-react";

// Storage key used by the no-FOUC script in app/layout.tsx too — keep in sync.
const STORAGE_KEY = "citadel-theme";

type Theme = "light" | "dark";

export function ThemeToggle() {
  const [theme, setTheme] = useState<Theme>("light");
  const [mounted, setMounted] = useState(false);

  // After hydration, read whatever the no-FOUC script applied (so the icon
  // matches the actual current theme). Doing it in useEffect avoids
  // hydration-mismatch warnings — server-render is always the light icon.
  useEffect(() => {
    const isDark = document.documentElement.classList.contains("dark");
    setTheme(isDark ? "dark" : "light");
    setMounted(true);
  }, []);

  const toggle = () => {
    const next: Theme = theme === "light" ? "dark" : "light";
    setTheme(next);
    if (next === "dark") {
      document.documentElement.classList.add("dark");
    } else {
      document.documentElement.classList.remove("dark");
    }
    try {
      localStorage.setItem(STORAGE_KEY, next);
    } catch {
      // ignore — private browsing / quota
    }
  };

  // Render an icon-only button. Until mount, show the light-mode icon
  // (matches SSR output) — avoids a flash of mismatched icon.
  return (
    <button
      onClick={toggle}
      aria-label={`Switch to ${theme === "light" ? "dark" : "light"} mode`}
      title={`Switch to ${theme === "light" ? "dark" : "light"} mode`}
      className="rounded p-1.5 text-ink-muted hover:text-brand-600 hover:bg-brand-50"
      suppressHydrationWarning
    >
      {!mounted || theme === "light" ? (
        <Moon className="h-4 w-4" />
      ) : (
        <Sun className="h-4 w-4" />
      )}
    </button>
  );
}
