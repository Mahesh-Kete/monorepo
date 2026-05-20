"use client";

import { useMemo } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import {
  ArrowLeft, FolderGit2, ExternalLink, Activity,
  Globe2, FileText, AlertTriangle, GitCommit,
} from "lucide-react";
import { api } from "@/lib/api";
import type {
  CitadelEvent, ConnectedRepo, RunDetail, RunSummary,
} from "@/lib/types";
import {
  ModeBadge, RunStatusBadge, SeverityBadge, StatusPill,
} from "@/components/badges";
import { LiveIndicator } from "@/components/live-indicator";
import { useLivePoll } from "@/lib/use-live-poll";

export default function RepoDetailPage() {
  const params = useParams<{ id: string }>();
  const repoId = Number(params.id);

  // Poll the repo list every 10s — slow, since repos rarely change.
  const { data: repos } = useLivePoll<ConnectedRepo[]>(
    () => api.listRepos(),
    10000,
  );
  // Poll the runs list every 3s — runs change often when CI is active.
  const { data: runs, updatedAt: runsUpdatedAt } = useLivePoll<RunSummary[]>(
    () => api.listRuns(200),
    3000,
  );

  const repo = repos?.find((r) => r.id === repoId) ?? null;
  const repoRuns = useMemo(
    () => (repo && runs ? runs.filter((r) => r.repository === repo.repository) : []),
    [repo, runs],
  );
  const latestRun = repoRuns[0] ?? null;

  // Live poll the latest run's detail every 2s for the streaming event view.
  const { data: latestDetail } = useLivePoll<RunDetail>(
    () => api.getRun(latestRun!.id),
    2000,
    !!latestRun,
  );

  if (repos !== null && !repo) return <NotFound repoId={repoId} />;
  if (!repo) return <div className="text-ink-subtle">Loading…</div>;

  return (
    <div>
      {/* Back link */}
      <Link
        href="/repos"
        className="inline-flex items-center gap-1.5 text-sm text-ink-muted hover:text-brand-600 mb-3"
      >
        <ArrowLeft className="h-3.5 w-3.5" /> All repositories
      </Link>

      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-baseline gap-3">
          <h1 className="text-xl font-semibold tracking-tight flex items-center gap-2">
            <FolderGit2 className="h-5 w-5 text-brand-600" />
            <span className="mono">{repo.repository}</span>
            <a
              href={`https://github.com/${repo.repository}`}
              target="_blank" rel="noopener noreferrer"
              className="text-ink-subtle hover:text-brand-600"
              title="Open on GitHub"
            >
              <ExternalLink className="h-3.5 w-3.5" />
            </a>
          </h1>
          <LiveIndicator updatedAt={runsUpdatedAt} />
        </div>
      </div>

      <RepoMeta repo={repo} runCount={repoRuns.length} />

      {/* Latest run — the "live CI/CD pipeline" view */}
      <div className="mt-5">
        <h2 className="text-sm font-medium text-ink-muted uppercase tracking-wide mb-2">
          Current / latest run
        </h2>
        {latestRun ? (
          <LatestRunCard run={latestRun} detail={latestDetail} />
        ) : (
          <div className="rounded-md border border-surface-line bg-surface-card p-6 text-center text-ink-subtle text-sm">
            No runs yet. Citadel polls GitHub every 30 s; once a workflow runs it'll show up here.
          </div>
        )}
      </div>

      {/* Live event stream */}
      {latestDetail && latestDetail.events.length > 0 && (
        <div className="mt-5">
          <h2 className="text-sm font-medium text-ink-muted uppercase tracking-wide mb-2">
            Live event stream
            <span className="ml-2 text-xs text-ink-subtle normal-case">
              from run #{latestRun!.run_number || latestRun!.id} · refreshes every 2 s
            </span>
          </h2>
          <LiveEventStream events={latestDetail.events} />
        </div>
      )}

      {/* Detections across this repo */}
      <div className="mt-5">
        <h2 className="text-sm font-medium text-ink-muted uppercase tracking-wide mb-2">
          Recent detections
        </h2>
        <RepoDetections repoRuns={repoRuns} />
      </div>

      {/* Run history */}
      <div className="mt-5">
        <h2 className="text-sm font-medium text-ink-muted uppercase tracking-wide mb-2">
          Run history
          <span className="ml-2 text-xs text-ink-subtle normal-case">
            {repoRuns.length} total
          </span>
        </h2>
        <RunHistoryTable runs={repoRuns} />
      </div>
    </div>
  );
}

// ============================================================================
// Pieces
// ============================================================================

function RepoMeta({ repo, runCount }: { repo: ConnectedRepo; runCount: number }) {
  return (
    <div className="rounded-md border border-surface-line bg-surface-card shadow-card p-3">
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3 text-sm">
        <Meta label="Connected">
          {new Date(repo.created_at).toLocaleDateString()}
        </Meta>
        <Meta label="Last polled">
          {repo.last_polled_at
            ? new Date(repo.last_polled_at).toLocaleString()
            : "—"}
        </Meta>
        <Meta label="Total runs">
          <span className="mono">{runCount}</span>
        </Meta>
        <Meta label="Status">
          {repo.last_error ? (
            <span className="inline-flex items-center gap-1.5 text-block-700">
              <AlertTriangle className="h-3.5 w-3.5" />
              <span className="mono text-xs">{repo.last_error}</span>
            </span>
          ) : (
            <span className="text-ok-700">healthy</span>
          )}
        </Meta>
      </div>
      {repo.note && (
        <div className="mt-3 pt-3 border-t border-surface-line text-xs text-ink-muted">
          {repo.note}
        </div>
      )}
    </div>
  );
}

function Meta({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <div className="text-xs uppercase text-ink-muted tracking-wide">{label}</div>
      <div className="mt-0.5 text-ink">{children}</div>
    </div>
  );
}

function LatestRunCard({ run, detail }: { run: RunSummary; detail: RunDetail | null }) {
  const events = detail?.events ?? [];
  const netCount = events.filter((e) => e.type === "network").length;
  const fileCount = events.filter((e) => e.type === "file" || e.type === "file_tamper").length;
  const blocked = events.filter((e) => e.network?.blocked === true).length;
  const isLive = run.status === "in_progress" || run.gh_status === "in_progress";

  return (
    <Link
      href={`/runs/${run.id}`}
      className="block rounded-md border border-surface-line bg-surface-card shadow-card p-4 hover:border-brand-500/60 transition"
    >
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
        <RunStatusBadge status={run.gh_conclusion || run.gh_status || run.status} />
        <span className="font-medium">{run.workflow || "(no workflow)"}</span>
        <span className="text-ink-muted">·</span>
        <span className="mono text-ink-muted text-sm">
          {run.run_number ? `#${run.run_number}` : `run ${run.run_id}`}
        </span>
        <ModeBadge mode={run.policy_mode} />
        {isLive && (
          <span className="inline-flex items-center gap-1 text-xs text-brand-700">
            <Activity className="h-3.5 w-3.5 animate-pulse" />
            running
          </span>
        )}
        {run.severity_max && (
          <span className="ml-auto">
            <SeverityBadge severity={run.severity_max} />
          </span>
        )}
      </div>

      <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-ink-muted">
        {run.sha && (
          <span className="inline-flex items-center gap-1 mono">
            <GitCommit className="h-3 w-3" /> {run.sha.slice(0, 12)}
          </span>
        )}
        {run.actor && <span>by {run.actor}</span>}
        <span>{new Date(run.started_at).toLocaleString()}</span>
      </div>

      <div className="mt-3 grid grid-cols-3 gap-2">
        <Tile icon={<Globe2 className="h-3.5 w-3.5" />} label="Network" value={netCount} />
        <Tile icon={<FileText className="h-3.5 w-3.5" />} label="Files" value={fileCount} />
        <Tile
          icon={<AlertTriangle className="h-3.5 w-3.5" />}
          label="Blocked"
          value={blocked}
          accent={blocked > 0 ? "block" : "neutral"}
        />
      </div>
    </Link>
  );
}

function Tile({
  icon, label, value, accent = "neutral",
}: {
  icon: React.ReactNode; label: string; value: number; accent?: "neutral" | "block";
}) {
  const accentCls =
    accent === "block" ? "text-block-700" : "text-ink";
  return (
    <div className="rounded border border-surface-line bg-surface-rail/40 px-2.5 py-2">
      <div className="flex items-center gap-1.5 text-xs uppercase text-ink-muted tracking-wide">
        {icon} {label}
      </div>
      <div className={`mt-0.5 text-lg font-semibold mono ${accentCls}`}>{value}</div>
    </div>
  );
}

function LiveEventStream({ events }: { events: CitadelEvent[] }) {
  // Newest first, capped at 100 to keep the page snappy.
  const rows = [...events]
    .filter((e) => e.type !== "process")
    .sort((a, b) => +new Date(b.timestamp) - +new Date(a.timestamp))
    .slice(0, 100);

  return (
    <div className="rounded-md border border-surface-line bg-surface-card shadow-card overflow-hidden">
      <table className="w-full text-sm">
        <thead className="bg-surface-rail text-ink-muted text-xs uppercase tracking-wide">
          <tr>
            <th className="px-3 py-2 text-left font-medium">Time</th>
            <th className="px-3 py-2 text-left font-medium">Type</th>
            <th className="px-3 py-2 text-left font-medium">Process</th>
            <th className="px-3 py-2 text-left font-medium">Detail</th>
            <th className="px-3 py-2 text-left font-medium">Status</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-surface-line">
          {rows.length === 0 ? (
            <tr>
              <td colSpan={5} className="px-3 py-6 text-center text-ink-subtle">
                No events yet.
              </td>
            </tr>
          ) : (
            rows.map((e) => <EventRow key={e.id} event={e} />)
          )}
        </tbody>
      </table>
    </div>
  );
}

function EventRow({ event: e }: { event: CitadelEvent }) {
  return (
    <tr className="hover:bg-brand-50/40">
      <td className="px-3 py-1.5 mono text-xs text-ink-muted whitespace-nowrap">
        {new Date(e.timestamp).toLocaleTimeString()}
      </td>
      <td className="px-3 py-1.5">
        <TypeBadge type={e.type} />
      </td>
      <td className="px-3 py-1.5 mono text-xs">
        {e.network?.process || e.process?.comm || e.process_chain?.[0] || "—"}
      </td>
      <td className="px-3 py-1.5 mono text-xs break-all">
        <EventDetail event={e} />
      </td>
      <td className="px-3 py-1.5">
        {e.type === "network" ? (
          <StatusPill allowed={e.network?.blocked !== true} />
        ) : e.type === "file_tamper" ? (
          <StatusPill allowed={false} />
        ) : (
          <span className="text-xs text-ink-subtle">—</span>
        )}
      </td>
    </tr>
  );
}

function EventDetail({ event: e }: { event: CitadelEvent }) {
  if (e.type === "network") {
    const dst = e.network?.hostname || e.network?.dst_ip || "?";
    return (
      <span>
        → {dst}
        <span className="text-ink-subtle">:{e.network?.dst_port}</span>
      </span>
    );
  }
  if (e.type === "file" || e.type === "file_tamper") {
    return <span>{e.file?.path || "?"}</span>;
  }
  if (e.type === "process") {
    const args = (e.process?.args ?? []).slice(1).join(" ");
    return (
      <span>
        {e.process?.filename || e.process?.comm}
        {args && <span className="text-ink-subtle"> {args}</span>}
      </span>
    );
  }
  return <span className="text-ink-subtle">{e.type}</span>;
}

function TypeBadge({ type }: { type: string }) {
  const map: Record<string, { label: string; cls: string }> = {
    network:     { label: "net",    cls: "bg-brand-50  text-brand-700 border-brand-500/30" },
    file:        { label: "file",   cls: "bg-ok-50     text-ok-700    border-ok-500/30" },
    file_tamper: { label: "tamper", cls: "bg-block-50  text-block-700 border-block-500/30" },
    process:     { label: "proc",   cls: "bg-surface-rail text-ink     border-surface-line" },
  };
  const m = map[type] || { label: type, cls: "bg-surface-rail text-ink-muted border-surface-line" };
  return (
    <span className={`inline-flex items-center rounded border px-1.5 py-0.5 text-xs mono ${m.cls}`}>
      {m.label}
    </span>
  );
}

function RepoDetections({ repoRuns }: { repoRuns: RunSummary[] }) {
  // Use the data already returned by /api/runs (detection_count + severity_max
  // per run). Listing every individual detection across the repo would need a
  // separate API call — keep it lightweight here and link out to /runs/[id].
  const withDetections = repoRuns.filter((r) => r.detection_count > 0).slice(0, 10);

  if (withDetections.length === 0) {
    return (
      <div className="rounded-md border border-surface-line bg-surface-card p-4 text-sm text-ink-subtle">
        No detections in any run yet.
      </div>
    );
  }
  return (
    <div className="rounded-md border border-surface-line bg-surface-card shadow-card overflow-hidden">
      <ul className="divide-y divide-surface-line">
        {withDetections.map((r) => (
          <li key={r.id}>
            <Link
              href={`/runs/${r.id}`}
              className="flex items-center gap-3 px-3 py-2.5 hover:bg-brand-50/40"
            >
              {r.severity_max && <SeverityBadge severity={r.severity_max} />}
              <span className="mono text-sm">
                {r.workflow || "(no workflow)"}
                {r.run_number && <span className="text-ink-muted"> · #{r.run_number}</span>}
              </span>
              <span className="text-sm text-ink-muted">
                {r.detection_count} finding{r.detection_count > 1 ? "s" : ""}
              </span>
              <span className="ml-auto text-xs text-ink-subtle">
                {new Date(r.started_at).toLocaleString()}
              </span>
            </Link>
          </li>
        ))}
      </ul>
    </div>
  );
}

function RunHistoryTable({ runs }: { runs: RunSummary[] }) {
  if (runs.length === 0) {
    return (
      <div className="rounded-md border border-surface-line bg-surface-card p-4 text-sm text-ink-subtle">
        No runs to show.
      </div>
    );
  }
  return (
    <div className="rounded-md border border-surface-line bg-surface-card shadow-card overflow-hidden">
      <table className="w-full text-sm">
        <thead className="bg-surface-rail text-ink-muted text-xs uppercase tracking-wide">
          <tr>
            <th className="px-3 py-2 text-left font-medium">Status</th>
            <th className="px-3 py-2 text-left font-medium">Workflow</th>
            <th className="px-3 py-2 text-left font-medium">Run</th>
            <th className="px-3 py-2 text-left font-medium">Policy Mode</th>
            <th className="px-3 py-2 text-left font-medium">Events</th>
            <th className="px-3 py-2 text-left font-medium">Findings</th>
            <th className="px-3 py-2 text-left font-medium">Started</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-surface-line">
          {runs.map((r) => {
            const totalEvents =
              r.event_counts.network + r.event_counts.process +
              r.event_counts.file + r.event_counts.file_tamper;
            return (
              <tr key={r.id} className="hover:bg-brand-50/40">
                <td className="px-3 py-2">
                  <Link href={`/runs/${r.id}`} className="inline-flex items-center">
                    <RunStatusBadge status={r.gh_conclusion || r.gh_status || r.status} />
                  </Link>
                </td>
                <td className="px-3 py-2">
                  <Link href={`/runs/${r.id}`} className="mono text-brand-600 hover:underline">
                    {r.workflow || "(no workflow)"}
                  </Link>
                </td>
                <td className="px-3 py-2 mono text-xs text-ink-muted">
                  {r.run_number ? `#${r.run_number}` : r.run_id}
                </td>
                <td className="px-3 py-2"><ModeBadge mode={r.policy_mode} /></td>
                <td className="px-3 py-2 mono">{totalEvents}</td>
                <td className="px-3 py-2">
                  {r.detection_count > 0 ? (
                    <span className="inline-flex items-center gap-2">
                      <span className="mono">{r.detection_count}</span>
                      {r.severity_max && <SeverityBadge severity={r.severity_max} />}
                    </span>
                  ) : (
                    <span className="text-ink-subtle">0</span>
                  )}
                </td>
                <td className="px-3 py-2 text-xs text-ink-muted whitespace-nowrap">
                  {new Date(r.started_at).toLocaleString()}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

function NotFound({ repoId }: { repoId: number }) {
  return (
    <div className="rounded-md border border-block-500/40 bg-block-50 p-4 text-sm text-block-700">
      Repository <span className="mono">#{repoId}</span> not found.
      {" "}<Link href="/repos" className="text-brand-600 hover:underline">Back to repositories</Link>
    </div>
  );
}
