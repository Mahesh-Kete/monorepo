"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { Globe2, Cpu, FileText, ShieldAlert, RefreshCw, Shield } from "lucide-react";
import { api } from "@/lib/api";
import type { RunSummary } from "@/lib/types";
import { CountChip, ModeBadge, SeverityBadge } from "@/components/badges";

function relativeTime(iso: string): string {
  const t = new Date(iso).getTime();
  const diff = Math.max(0, Date.now() - t);
  const sec = Math.floor(diff / 1000);
  if (sec < 60) return `${sec}s ago`;
  if (sec < 3600) return `${Math.floor(sec / 60)}m ago`;
  if (sec < 86400) return `${Math.floor(sec / 3600)}h ago`;
  return `${Math.floor(sec / 86400)}d ago`;
}

export default function RunsPage() {
  const [runs, setRuns] = useState<RunSummary[] | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [refreshing, setRefreshing] = useState(false);

  const load = async () => {
    setRefreshing(true);
    try {
      setRuns(await api.listRuns(50));
      setErr(null);
    } catch (e) {
      setErr(String(e));
    } finally {
      setRefreshing(false);
    }
  };
  useEffect(() => {
    load();
  }, []);

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-semibold tracking-tight">Workflow Runs</h1>
        <button
          onClick={load}
          disabled={refreshing}
          className="inline-flex items-center gap-2 rounded border border-slate-700 px-3 py-1.5 text-sm text-slate-300 hover:bg-slate-800 disabled:opacity-50"
        >
          <RefreshCw className={`h-4 w-4 ${refreshing ? "animate-spin" : ""}`} />
          Refresh
        </button>
      </div>

      {err && (
        <div className="rounded border border-red-800 bg-red-950/40 p-3 text-sm text-red-300 mono mb-4">
          {err}
        </div>
      )}

      {runs === null ? (
        <div className="text-slate-400">Loading…</div>
      ) : runs.length === 0 ? (
        <EmptyState />
      ) : (
        <div className="rounded-lg border border-slate-800 overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-slate-900/60 text-slate-400 text-xs uppercase tracking-wide">
              <tr>
                <th className="px-3 py-2 text-left">Repo</th>
                <th className="px-3 py-2 text-left">Workflow</th>
                <th className="px-3 py-2 text-left">Run</th>
                <th className="px-3 py-2 text-left">SHA</th>
                <th className="px-3 py-2 text-left">Mode</th>
                <th className="px-3 py-2 text-left">Events</th>
                <th className="px-3 py-2 text-left">Detections</th>
                <th className="px-3 py-2 text-left">Started</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800">
              {runs.map((r) => (
                <tr key={r.id} className="hover:bg-slate-900/40 transition-colors">
                  <td className="px-3 py-2">
                    <Link href={`/runs/${r.id}`} className="text-cyan-400 hover:underline">
                      {r.repository}
                    </Link>
                  </td>
                  <td className="px-3 py-2 text-slate-300">{r.workflow ?? "—"}</td>
                  <td className="px-3 py-2 mono text-slate-300">{r.run_number ? `#${r.run_number}` : r.run_id}</td>
                  <td className="px-3 py-2 mono text-slate-400">{r.sha?.slice(0, 7) ?? "—"}</td>
                  <td className="px-3 py-2"><ModeBadge mode={r.policy_mode} /></td>
                  <td className="px-3 py-2">
                    <div className="flex items-center gap-3">
                      <CountChip icon={<Globe2 className="h-3.5 w-3.5 text-cyan-400" />} value={r.event_counts?.network ?? 0} label="network" />
                      <CountChip icon={<Cpu className="h-3.5 w-3.5 text-emerald-400" />} value={r.event_counts?.process ?? 0} label="process" />
                      <CountChip icon={<FileText className="h-3.5 w-3.5 text-amber-400" />} value={r.event_counts?.file ?? 0} label="file" />
                      <CountChip icon={<ShieldAlert className="h-3.5 w-3.5 text-red-400" />} value={r.event_counts?.file_tamper ?? 0} label="tamper" />
                    </div>
                  </td>
                  <td className="px-3 py-2">
                    {r.detection_count > 0 ? (
                      <span className="flex items-center gap-2">
                        <span className="mono text-slate-300">{r.detection_count}</span>
                        {r.severity_max && <SeverityBadge severity={r.severity_max} />}
                      </span>
                    ) : (
                      <span className="text-slate-500">—</span>
                    )}
                  </td>
                  <td className="px-3 py-2 text-slate-400">{relativeTime(r.started_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center py-16 text-center">
      <Shield className="h-12 w-12 text-slate-700 mb-3" />
      <p className="text-slate-300 mb-1">No runs yet.</p>
      <p className="text-slate-500 text-sm mono">Add the citadel-setup step to your workflow to start streaming events.</p>
    </div>
  );
}
