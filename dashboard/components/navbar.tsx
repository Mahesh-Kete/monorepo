"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { Shield, Activity, Lock } from "lucide-react";
import { api } from "@/lib/api";
import { ThemeToggle } from "./theme-toggle";

export function Navbar() {
  const [ok, setOk] = useState<boolean | null>(null);

  useEffect(() => {
    let cancelled = false;
    const tick = async () => {
      try {
        const h = await api.health();
        if (!cancelled) setOk(h.status === "ok");
      } catch {
        if (!cancelled) setOk(false);
      }
    };
    tick();
    const t = setInterval(tick, 5000);
    return () => {
      cancelled = true;
      clearInterval(t);
    };
  }, []);

  const dot = ok === null ? "bg-slate-400" : ok ? "bg-ok-500" : "bg-block-500";
  const dotLabel = ok === null ? "checking" : ok ? "connected" : "offline";

  return (
    <header className="border-b border-surface-line bg-surface-card sticky top-0 z-10">
      <div className="mx-auto max-w-7xl px-6 h-14 flex items-center gap-6">
        <Link href="/runs" className="flex items-center gap-2 hover:text-brand-600 transition-colors">
          <Shield className="h-5 w-5 text-brand-600" />
          <span className="text-base font-semibold tracking-tight">Citadel</span>
        </Link>
        <nav className="flex items-center gap-1 text-sm">
          <NavLink href="/runs"><Activity className="h-4 w-4" /> Runs</NavLink>
          <NavLink href="/policies"><Lock className="h-4 w-4" /> Policies</NavLink>
        </nav>
        <div className="ml-auto flex items-center gap-3 text-xs text-ink-muted">
          <span className="flex items-center gap-2">
            <span className={`inline-block h-2 w-2 rounded-full ${dot}`} />
            <span>backend · {dotLabel}</span>
          </span>
          <ThemeToggle />
        </div>
      </div>
    </header>
  );
}

function NavLink({ href, children }: { href: string; children: React.ReactNode }) {
  return (
    <Link
      href={href}
      className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded text-ink-muted hover:text-brand-600 hover:bg-brand-50"
    >
      {children}
    </Link>
  );
}
