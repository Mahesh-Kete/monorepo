"use client";

import { useEffect, useRef, useState } from "react";

// useLivePoll runs `loader` on mount and then on an interval. Pauses while
// the tab is hidden (visibility API) to avoid pointless work in background
// tabs. `loader` does NOT need to be memoized — we capture the latest
// reference via a ref so the polling interval never re-establishes itself.
export function useLivePoll<T>(
  loader: () => Promise<T>,
  intervalMs: number,
  enabled = true,
) {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [updatedAt, setUpdatedAt] = useState<Date | null>(null);
  const loaderRef = useRef(loader);
  loaderRef.current = loader;

  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;

    const tick = async () => {
      if (typeof document !== "undefined" && document.hidden) return;
      try {
        const r = await loaderRef.current();
        if (cancelled) return;
        setData(r);
        setUpdatedAt(new Date());
        setError(null);
      } catch (e) {
        if (cancelled) return;
        setError(String(e));
      }
    };

    tick();
    const t = setInterval(tick, intervalMs);

    // When tab becomes visible again, do an immediate refresh.
    const onVisible = () => {
      if (!document.hidden) tick();
    };
    document.addEventListener("visibilitychange", onVisible);

    return () => {
      cancelled = true;
      clearInterval(t);
      document.removeEventListener("visibilitychange", onVisible);
    };
  }, [intervalMs, enabled]);

  return { data, error, updatedAt, refetch: () => loaderRef.current() };
}
