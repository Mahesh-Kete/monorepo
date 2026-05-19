"use client";

import { useEffect, useState } from "react";

// LiveIndicator shows a pulsing green dot + relative-time-since-last-update,
// updated every second. Place it wherever you want to convey "streaming".
export function LiveIndicator({ updatedAt }: { updatedAt: Date | null }) {
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, []);

  if (!updatedAt) {
    return (
      <span className="inline-flex items-center gap-1.5 text-xs text-ink-subtle">
        <span className="inline-block h-2 w-2 rounded-full bg-slate-400" />
        Connecting…
      </span>
    );
  }

  const sec = Math.max(0, Math.floor((now - updatedAt.getTime()) / 1000));
  const stale = sec > 10;
  const dotCls = stale ? "bg-warn-500" : "bg-ok-500";

  return (
    <span className="inline-flex items-center gap-1.5 text-xs text-ink-muted">
      <span className="relative flex h-2 w-2">
        {!stale && (
          <span className={`absolute inline-flex h-full w-full animate-ping rounded-full ${dotCls} opacity-60`} />
        )}
        <span className={`relative inline-flex h-2 w-2 rounded-full ${dotCls}`} />
      </span>
      Live · updated {sec === 0 ? "just now" : `${sec}s ago`}
    </span>
  );
}
