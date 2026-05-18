"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { Shield, ScrollText, Activity, Lock } from "lucide-react";
import { api } from "@/lib/api";

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

  const dot = ok === null ? "bg-slate-500" : ok ? "bg-emerald-500" : "bg-red-500";

  return (
    <header className="border-b border-slate-800 bg-slate-950/80 backdrop-blur sticky top-0 z-10">
      <div className="mx-auto max-w-7xl px-4 py-3 flex items-center gap-6">
        <Link href="/runs" className="flex items-center gap-2 text-slate-100 hover:text-cyan-400 transition-colors">
          <Shield className="h-5 w-5 text-cyan-400" />
          <span className="text-lg font-semibold tracking-tight">Citadel</span>
        </Link>
        <nav className="flex items-center gap-4 text-sm">
          <Link href="/runs" className="flex items-center gap-1.5 text-slate-300 hover:text-cyan-400">
            <Activity className="h-4 w-4" /> Runs
          </Link>
          <Link href="/policies" className="flex items-center gap-1.5 text-slate-300 hover:text-cyan-400">
            <Lock className="h-4 w-4" /> Policies
          </Link>
        </nav>
        <div className="ml-auto flex items-center gap-2 text-xs text-slate-400">
          <span className={`inline-block h-2 w-2 rounded-full ${dot}`} />
          backend
        </div>
      </div>
    </header>
  );
}
