"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import {
  GitBranch, Search, RefreshCw, Shield, Trash2,
  ChevronDown, ChevronRight, FolderGit2,
} from "lucide-react";
import { api } from "@/lib/api";
import type { RunSummary } from "@/lib/types";
import { JobStatusDot, SeverityBadge } from "@/components/badges";
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

type RepoGroup = {
  repository: string;
  runs: RunSummary[];      // newest first
  latest: RunSummary;      // == runs[0]
  totalEvents: number;
  totalDetections: number;
  worstSeverity?: string;
};

const SEVERITY_RANK: Record<string, number> = {
  critical: 5, high: 4, medium: 3, low: 2, info: 1,
};

function groupByRepo(runs: RunSummary[]): RepoGroup[] {
  const byRepo = new Map<string, RunSummary[]>();
  for (const r of runs) {
    const arr = byRepo.get(r.repository) ?? [];
    arr.push(r);
    byRepo.set(r.repository, arr);
  }
  const groups: RepoGroup[] = [];
  for (const [repository, list] of byRepo) {
    list.sort((a, b) => +new Date(b.started_at) - +new Date(a.started_at));
    const totalEvents = list.reduce(
      (n, r) => n + r.event_counts.network + r.event_counts.process +
                    r.event_counts.file + r.event_counts.file_tamper,
      0,
    );
    const totalDetections = list.reduce((n, r) => n + r.detection_count, 0);
    let worst: string | undefined;
    let worstRank = 0;
    for (const r of list) {
      if (!r.severity_max) continue;
      const rank = SEVERITY_RANK[r.severity_max] ?? 0;
      if (rank > worstRank) { worst = r.severity_max; worstRank = rank; }
    }
    groups.push({
      repository,
      runs: list,
      latest: list[0],
      totalEvents,
      totalDetections,
      worstSeverity: worst,
    });
  }
  // Sort groups by latest run, newest first.
  groups.sort((a, b) => +new Date(b.latest.started_at) - +new Date(a.latest.started_at));
  return groups;
}

export default function RunsPage() {
  const { data: runs, error: err, updatedAt, refetch } = useLivePoll<RunSummary[]>(
    () => api.listRuns(200),
    3000,
  );
  const [filter, setFilter] = useState("");
  const [manualRefresh, setManualRefresh] = useState(false);
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});

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

  const groups = useMemo(() => filtered ? groupByRepo(filtered) : null, [filtered]);

  const onRefresh = async () => {
    setManualRefresh(true);
    try { await refetch(); } finally {
      setTimeout(() => setManualRefresh(false), 300);
    }
  };

  const unknownCount = useMemo(
    () => (runs ? runs.filter((r) => r.repository === "(unknown)").length : 0),
    [runs],
  );

  const onDelete = async (r: RunSummary) => {
    const label = r.repository === "(unknown)"
      ? `unknown run #${r.id}`
      : `${r.repository} run ${r.run_number ? `#${r.run_number}` : r.run_id}`;
    if (!confirm(`Delete ${label}? Its events and detections will also be removed.`)) return;
    try {
      await api.deleteRun(r.id);
      await refetch();
    } catch (e) {
      alert(`Failed to delete: ${(e as Error).message}`);
    }
  };

  const onCleanupUnknown = async () => {
    if (!confirm(`Delete all ${unknownCount} run(s) where repository is "(unknown)"? Their events and detections will also be removed.`)) return;
    try {
      const res = await api.deleteUnknownRuns();
      await refetch();
      alert(`Deleted ${res.deleted} run(s).`);
    } catch (e) {
      alert(`Failed to clean up: ${(e as Error).message}`);
    }
  };

  const totalRuns = filtered?.length ?? 0;

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-baseline gap-3">
          <h1 className="text-xl font-semibold tracking-tight">Workflow runs</h1>
          <LiveIndicator updatedAt={updatedAt} />
        </div>
        <div className="flex items-center gap-2">
          {unknownCount > 0 && (
            <button
              onClick={onCleanupUnknown}
              className="inline-flex items-center gap-1.5 rounded border border-block-500/40 bg-block-50 px-3 py-1.5 text-sm text-block-700 hover:bg-block-100"
              title='Delete all runs with repository="(unknown)"'
            >
              <Trash2 className="h-4 w-4" />
              Clean up unknown ({unknownCount})
            </button>
          )}
          <button
            onClick={onRefresh}
            disabled={manualRefresh}
            className="inline-flex items-center gap-1.5 rounded border border-surface-line bg-surface-card px-3 py-1.5 text-sm hover:bg-brand-50 hover:border-brand-200 disabled:opacity-50"
          >
            <RefreshCw className={`h-4 w-4 ${manualRefresh ? "animate-spin" : ""}`} />
            Refresh
          </button>
        </div>
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
        {groups && (
          <span className="text-sm text-ink-muted">
            {groups.length} repo{groups.length !== 1 ? "s" : ""} · {totalRuns} run{totalRuns !== 1 ? "s" : ""}
          </span>
        )}
      </div>

      {err && (
        <div className="rounded border border-block-500/40 bg-block-50 p-3 text-sm text-block-700 mono mb-4">
          {err}
        </div>
      )}

      {groups === null ? (
        <div className="text-ink-subtle">Loading…</div>
      ) : groups.length === 0 ? (
        runs && runs.length > 0 ? (
          <div className="text-ink-subtle p-8 text-center bg-surface-card rounded border border-surface-line">
            No runs match <span className="mono">{filter || "—"}</span>.
          </div>
        ) : (
          <EmptyState />
        )
      ) : (
        <div className="space-y-3">
          {groups.map((g) => (
            <RepoGroupCard
              key={g.repository}
              group={g}
              collapsed={!!collapsed[g.repository]}
              onToggle={() =>
                setCollapsed((c) => ({ ...c, [g.repository]: !c[g.repository] }))
              }
              onDelete={onDelete}
            />
          ))}
        </div>
      )}
    </div>
  );
}

function RepoGroupCard({
  group, collapsed, onToggle, onDelete,
}: {
  group: RepoGroup;
  collapsed: boolean;
  onToggle: () => void;
  onDelete: (r: RunSummary) => void;
}) {
  const { repository, runs, latest, totalEvents, totalDetections, worstSeverity } = group;
  const Chev = collapsed ? ChevronRight : ChevronDown;
  const isUnknown = repository === "(unknown)";

  return (
    <div className="rounded-md border border-surface-line bg-surface-card shadow-card overflow-hidden">
      <button
        onClick={onToggle}
        className="w-full flex items-center gap-3 px-3 py-2.5 hover:bg-brand-50/40 transition-colors text-left"
      >
        <Chev className="h-4 w-4 text-ink-subtle shrink-0" />
        <FolderGit2 className="h-4 w-4 text-brand-600 shrink-0" />
        {isUnknown ? (
          <span className="font-medium mono text-ink-muted italic">{repository}</span>
        ) : (
          <span className="font-medium mono">{repository}</span>
        )}
        <span className="text-xs text-ink-muted">
          {runs.length} run{runs.length !== 1 ? "s" : ""}
        </span>

        <span className="ml-3 flex items-center gap-2 text-xs">
          <JobStatusDot status={latest.gh_status || latest.status} />
          <span className="text-ink-muted">latest:</span>
          <span className="capitalize">
            {(latest.gh_conclusion || latest.gh_status || latest.status).replace("_", " ")}
          </span>
          <span className="text-ink-subtle">· {relativeTime(latest.started_at)}</span>
        </span>

        <span className="ml-auto flex items-center gap-3 text-xs text-ink-muted">
          <span className="mono">{totalEvents} events</span>
          {totalDetections > 0 ? (
            <span className="flex items-center gap-1.5">
              <span className="mono">{totalDetections} findings</span>
              {worstSeverity && <SeverityBadge severity={worstSeverity} />}
            </span>
          ) : (
            <span className="text-ink-subtle">no findings</span>
          )}
        </span>
      </button>

      {!collapsed && (
        <div className="border-t border-surface-line">
          <table className="w-full text-sm">
            <thead className="bg-surface-rail text-ink-muted text-xs uppercase tracking-wide">
              <tr>
                <Th>Status</Th>
                <Th>Workflow</Th>
                <Th>Run</Th>
                <Th>Commit</Th>
                <Th>Citadel</Th>
                <Th>Events</Th>
                <Th>Detections</Th>
                <Th>Started</Th>
                <Th></Th>
              </tr>
            </thead>
            <tbody className="divide-y divide-surface-line">
              {runs.map((r) => (
                <tr key={r.id} className="hover:bg-brand-50/40 transition-colors">
                  <Td>
                    <Link href={`/runs/${r.id}`} className="inline-flex items-center gap-1.5">
                      <JobStatusDot status={r.gh_status || r.status} />
                      <span className="text-xs text-ink-muted capitalize">
                        {(r.gh_conclusion || r.gh_status || r.status).replace("_", " ")}
                      </span>
                    </Link>
                  </Td>
                  <Td>
                    <Link href={`/runs/${r.id}`} className="text-brand-600 font-medium hover:underline">
                      {r.workflow ?? "—"}
                    </Link>
                  </Td>
                  <Td className="mono text-ink-muted">
                    {r.run_number ? `#${r.run_number}` : r.run_id}
                  </Td>
                  <Td>
                    {r.sha ? (
                      <span className="inline-flex items-center gap-1 mono text-ink-muted">
                        <GitBranch className="h-3 w-3" />
                        {r.sha.slice(0, 7)}
                      </span>
                    ) : "—"}
                  </Td>
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
                  <Td className="text-right">
                    <button
                      onClick={(e) => { e.preventDefault(); onDelete(r); }}
                      title="Delete this run"
                      aria-label="Delete run"
                      className="rounded p-1 text-ink-subtle hover:bg-block-50 hover:text-block-700"
                    >
                      <Trash2 className="h-4 w-4" />
                    </button>
                  </Td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function Th({ children }: { children?: React.ReactNode }) {
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
