"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { GitBranch, Search, RefreshCw, Shield } from "lucide-react";
import { api } from "@/lib/api";
import type { RunSummary } from "@/lib/types";
import { JobStatusDot, ModeBadge, SeverityBadge } from "@/components/badges";
import { LiveIndicator } from "@/components/live-indicator";
import { useLivePoll } from "@/lib/use-live-poll";

function relativeTime(iso: string): string {
  const diff = Math.max(0, Date.now() - new Date(iso).getTime());
  const sec = Math.floor(diff / 1000);
  if (sec < 60) return `${sec}s ago`;
  if (sec < 3600) return `${Math.floor(sec / 60)}m ago`;
  if (sec < 86400) return `${Math.floor(sec / 3600)}h ago`;
  return `${Math.floor(sec / 86400)}d ago`;
}

export default function RunsPage() {
  const { data: runs, error: err, updatedAt, refetch } = useLivePoll<RunSummary[]>(
    () => api.listRuns(50),
    3000,
  );
  const [filter, setFilter] = useState("");
  const [manualRefresh, setManualRefresh] = useState(false);

  const filtered = useMemo(() => {
    if (!runs) return null;
    if (!filter) return runs;
    const s = filter.toLowerCase();
    return runs.filter((r) =>
      [r.repository, r.workflow, r.actor, r.sha, r.run_id, r.run_number]
        .filter(Boolean)
        .some((v) => (v as string).toLowerCase().includes(s)),
    );
  }, [runs, filter]);

  const onRefresh = async () => {
    setManualRefresh(true);
    try { await refetch(); } finally {
      // small delay so the spin animation reads
      setTimeout(() => setManualRefresh(false), 300);
    }
  };

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-baseline gap-3">
          <h1 className="text-xl font-semibold tracking-tight">Workflow runs</h1>
          <LiveIndicator updatedAt={updatedAt} />
        </div>
        <button
          onClick={onRefresh}
          disabled={manualRefresh}
          className="inline-flex items-center gap-1.5 rounded border border-surface-line bg-surface-card px-3 py-1.5 text-sm hover:bg-brand-50 hover:border-brand-200 disabled:opacity-50"
        >
          <RefreshCw className={`h-4 w-4 ${manualRefresh ? "animate-spin" : ""}`} />
          Refresh
        </button>
      </div>

      <div className="mb-3 flex items-center gap-2">
        <div className="relative w-72">
          <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-ink-subtle" />
          <input
            placeholder="filter by repo, workflow, actor, sha…"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            className="w-full rounded border border-surface-line bg-surface-card pl-8 pr-3 py-1.5 text-sm focus:outline-none focus:border-brand-500"
          />
        </div>
        {filtered && <span className="text-sm text-ink-muted">{filtered.length} run{filtered.length !== 1 ? "s" : ""}</span>}
      </div>

      {err && (
        <div className="rounded border border-block-500/40 bg-block-50 p-3 text-sm text-block-700 mono mb-4">
          {err}
        </div>
      )}

      {filtered === null ? (
        <div className="text-ink-subtle">Loading…</div>
      ) : filtered.length === 0 ? (
        runs && runs.length > 0 ? (
          <div className="text-ink-subtle p-8 text-center bg-surface-card rounded border border-surface-line">
            No runs match <span className="mono">{filter || "—"}</span>.
          </div>
        ) : (
          <EmptyState />
        )
      ) : (
        <div className="rounded-md border border-surface-line bg-surface-card shadow-card overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-surface-rail text-ink-muted text-xs uppercase tracking-wide">
              <tr>
                <Th>Status</Th>
                <Th>Repository</Th>
                <Th>Workflow</Th>
                <Th>Run</Th>
                <Th>Commit</Th>
                <Th>Mode</Th>
                <Th>Citadel</Th>
                <Th>Events</Th>
                <Th>Detections</Th>
                <Th>Started</Th>
              </tr>
            </thead>
            <tbody className="divide-y divide-surface-line">
              {filtered.map((r) => (
                <tr key={r.id} className="hover:bg-brand-50/40 transition-colors">
                  <Td>
                    <span className="inline-flex items-center gap-1.5">
                      <JobStatusDot status={r.gh_status || r.status} />
                      <span className="text-xs text-ink-muted capitalize">
                        {(r.gh_conclusion || r.gh_status || r.status).replace("_", " ")}
                      </span>
                    </span>
                  </Td>
                  <Td>
                    <Link href={`/runs/${r.id}`} className="text-brand-600 font-medium hover:underline">
                      {r.repository}
                    </Link>
                  </Td>
                  <Td>{r.workflow ?? "—"}</Td>
                  <Td className="mono text-ink-muted">{r.run_number ? `#${r.run_number}` : r.run_id}</Td>
                  <Td>
                    {r.sha ? (
                      <span className="inline-flex items-center gap-1 mono text-ink-muted">
                        <GitBranch className="h-3 w-3" />
                        {r.sha.slice(0, 7)}
                      </span>
                    ) : "—"}
                  </Td>
                  <Td><ModeBadge mode={r.policy_mode} /></Td>
                  <Td>
                    {r.agent_seen ? (
                      <span className="inline-flex items-center gap-1 rounded border border-ok-500/30 bg-ok-50 px-1.5 py-0.5 text-[10px] font-medium text-ok-700 uppercase">
                        Citadel
                      </span>
                    ) : (
                      <span className="inline-flex items-center gap-1 rounded border border-surface-line bg-surface-rail px-1.5 py-0.5 text-[10px] font-medium text-ink-subtle uppercase">
                        Agent not installed
                      </span>
                    )}
                  </Td>
                  <Td>
                    <div className="flex items-center gap-2 mono text-xs text-ink-muted">
                      <span title="network">🌐 {r.event_counts.network}</span>
                      <span title="process">🧬 {r.event_counts.process}</span>
                      <span title="file">📝 {r.event_counts.file + r.event_counts.file_tamper}</span>
                    </div>
                  </Td>
                  <Td>
                    {r.detection_count > 0 ? (
                      <span className="flex items-center gap-2">
                        <span className="mono">{r.detection_count}</span>
                        {r.severity_max && <SeverityBadge severity={r.severity_max} />}
                      </span>
                    ) : (
                      <span className="text-ink-subtle">—</span>
                    )}
                  </Td>
                  <Td className="text-ink-muted">{relativeTime(r.started_at)}</Td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function Th({ children }: { children: React.ReactNode }) {
  return <th className="px-3 py-2 text-left font-medium">{children}</th>;
}

function Td({ children, className = "" }: { children: React.ReactNode; className?: string }) {
  return <td className={`px-3 py-2 align-middle ${className}`}>{children}</td>;
}

function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center py-20 text-center bg-surface-card rounded border border-surface-line">
      <Shield className="h-12 w-12 text-ink-subtle mb-3" />
      <p className="text-ink font-medium">No runs yet.</p>
      <p className="text-ink-muted text-sm mt-1">Add the <span className="mono">citadel-setup</span> step to your workflow to start streaming events.</p>
    </div>
  );
}
