"use client";

import { useState } from "react";
import { useParams } from "next/navigation";
import {
  Globe2, FileText, Cpu, LayoutDashboard, AlertTriangle,
  GitCommit, User, Calendar, Hash,
} from "lucide-react";
import { api, type ProcessTreeNode } from "@/lib/api";
import type { CitadelEvent, DetectionRow, GitHubActionLogRow, RunDetail } from "@/lib/types";
import {
  JobStatusDot, ModeBadge, SeverityBadge, StatusPill,
} from "@/components/badges";
import { LiveIndicator } from "@/components/live-indicator";
import { useLivePoll } from "@/lib/use-live-poll";

type Tab = "summary" | "network" | "files" | "processes" | "action_logs";

const TABS: { id: Tab; label: string; icon: React.ReactNode }[] = [
  { id: "summary",         label: "Summary",         icon: <LayoutDashboard className="h-4 w-4" /> },
  { id: "network",         label: "Network Events",  icon: <Globe2 className="h-4 w-4" /> },
  { id: "files",           label: "File Write Events", icon: <FileText className="h-4 w-4" /> },
  { id: "processes",       label: "Process Events",  icon: <Cpu className="h-4 w-4" /> },
  { id: "action_logs",     label: "Action Logs",     icon: <AlertTriangle className="h-4 w-4" /> },
];

export default function RunDetailPage() {
  const params = useParams<{ id: string }>();
  const runId = Number(params.id);
  const [tab, setTab] = useState<Tab>("summary");

  // Poll run detail every 2s (events + detections refresh as the workflow runs).
  // Process tree polls every 5s (changes less frequently).
  const { data, error: err, updatedAt } = useLivePoll<RunDetail>(
    () => api.getRun(runId),
    2000,
    !!runId,
  );
  const { data: tree } = useLivePoll<ProcessTreeNode[]>(
    () => api.getProcessTree(runId),
    5000,
    !!runId,
  );

  if (err) return <ErrorBox msg={err} />;
  if (!data) return <div className="text-ink-subtle">Loading…</div>;

  const { run } = data;
  const events = data.events ?? [];
  const detections = data.detections ?? [];
  const byType = (t: string) => events.filter((e) => e.type === t);

  return (
    <div>
      <div className="mb-3 flex items-center justify-end">
        <LiveIndicator updatedAt={updatedAt} />
      </div>
      <RunHeader run={run} />

      {/* Two-column: left job rail + main content */}
      <div className="mt-5 grid grid-cols-12 gap-5">
        <aside className="col-span-12 md:col-span-3 lg:col-span-2">
          <JobRail run={run} />
        </aside>

        <section className="col-span-12 md:col-span-9 lg:col-span-10">
          <Tabs current={tab} onChange={setTab} />

          <div className="mt-4">
            {tab === "summary" && <SummaryTab run={run} events={events} detections={detections} />}
            {tab === "network" && <NetworkTab events={byType("network")} />}
            {tab === "files" && <FilesTab events={[...byType("file"), ...byType("file_tamper")]} />}
            {tab === "processes" && <ProcessTab tree={tree} flat={byType("process")} />}
            {tab === "action_logs" && <ActionLogsTab logs={data.action_logs ?? []} />}
          </div>
        </section>
      </div>
    </div>
  );
}

// ============================================================================
// Run header — repo / workflow / commit metadata
// ============================================================================

function RunHeader({ run }: { run: RunDetail["run"] }) {
  return (
    <div className="rounded-md border border-surface-line bg-surface-card shadow-card p-4">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
        <JobStatusDot status={run.status} />
        <h1 className="text-lg font-semibold">{run.repository}</h1>
        <span className="text-ink-muted">·</span>
        <span className="text-ink-muted">{run.workflow ?? "(no workflow)"}</span>
        <span className="text-ink-muted">·</span>
        <span className="mono text-ink-muted text-sm">
          {run.run_number ? `#${run.run_number}` : `run ${run.run_id}`}
        </span>
        <ModeBadge mode={run.policy_mode} />
      </div>
      <div className="mt-2.5 flex flex-wrap items-center gap-x-5 gap-y-1 text-sm text-ink-muted">
        {run.sha && (
          <MetaItem icon={<GitCommit className="h-3.5 w-3.5" />} mono>
            {run.sha.slice(0, 12)}
          </MetaItem>
        )}
        {run.actor && <MetaItem icon={<User className="h-3.5 w-3.5" />}>{run.actor}</MetaItem>}
        {run.ref && <MetaItem icon={<Hash className="h-3.5 w-3.5" />} mono>{run.ref}</MetaItem>}
        <MetaItem icon={<Calendar className="h-3.5 w-3.5" />}>
          {new Date(run.started_at).toLocaleString()}
        </MetaItem>
      </div>
    </div>
  );
}

function MetaItem({ icon, children, mono = false }: { icon: React.ReactNode; children: React.ReactNode; mono?: boolean }) {
  return (
    <span className={`inline-flex items-center gap-1 ${mono ? "mono" : ""}`}>
      {icon} {children}
    </span>
  );
}

// ============================================================================
// Left job rail — shows the run's job(s) and quick counts
// ============================================================================

function JobRail({ run }: { run: RunDetail["run"] }) {
  const counts = run.event_counts;
  return (
    <div className="rounded-md border border-surface-line bg-surface-rail p-3 sticky top-20">
      <h2 className="text-xs font-medium text-ink-muted uppercase tracking-wide mb-2">Job</h2>
      <div className="rounded bg-surface-card border border-surface-line p-2.5">
        <div className="flex items-center gap-2">
          <JobStatusDot status={run.status} />
          <span className="text-sm font-medium">build</span>
        </div>
        <div className="mt-1 text-xs text-ink-muted capitalize">{run.status.replace("_", " ")}</div>
      </div>

      <h2 className="mt-4 text-xs font-medium text-ink-muted uppercase tracking-wide mb-2">Quick counts</h2>
      <ul className="space-y-1.5 text-sm">
        <RailRow label="Network" count={counts.network} />
        <RailRow label="Files" count={counts.file + counts.file_tamper} />
        <RailRow label="Processes" count={counts.process} />
        <RailRow label="Detections" count={run.detection_count} severity={run.severity_max} />
      </ul>
    </div>
  );
}

function RailRow({ label, count, severity }: { label: string; count: number; severity?: string }) {
  return (
    <li className="flex items-center justify-between">
      <span className="text-ink-muted">{label}</span>
      <span className="flex items-center gap-2">
        <span className="mono text-ink">{count}</span>
        {severity && <SeverityBadge severity={severity} />}
      </span>
    </li>
  );
}

// ============================================================================
// Tabs
// ============================================================================

function Tabs({ current, onChange }: { current: Tab; onChange: (t: Tab) => void }) {
  return (
    <div className="border-b border-surface-line flex gap-1 overflow-x-auto">
      {TABS.map((t) => {
        const active = t.id === current;
        return (
          <button
            key={t.id}
            onClick={() => onChange(t.id)}
            className={
              "inline-flex items-center gap-2 px-3 py-2 text-sm whitespace-nowrap border-b-2 -mb-px " +
              (active
                ? "border-brand-600 text-brand-700 font-medium"
                : "border-transparent text-ink-muted hover:text-ink hover:border-surface-line")
            }
          >
            {t.icon}
            {t.label}
          </button>
        );
      })}
    </div>
  );
}

// ============================================================================
// Summary tab — overview tiles + recent detections
// ============================================================================

function SummaryTab({ run, events, detections }: { run: RunDetail["run"]; events: CitadelEvent[]; detections: DetectionRow[] }) {
  const netEvents = events.filter((e) => e.type === "network");
  const allowed = netEvents.filter((e) => e.network?.blocked !== true).length;
  const blocked = netEvents.filter((e) => e.network?.blocked === true).length;
  const distinctEndpoints = new Set(netEvents.map((e) => e.network?.hostname || e.network?.dst_ip)).size;
  const fileWrites = events.filter((e) => e.type === "file" || e.type === "file_tamper").length;
  const procExecs = events.filter((e) => e.type === "process").length;

  return (
    <div className="space-y-5">
      <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
        <Tile label="Distinct endpoints" value={distinctEndpoints} accent="brand" />
        <Tile label="Allowed calls" value={allowed} accent="ok" />
        <Tile label="Blocked calls" value={blocked} accent={blocked > 0 ? "block" : "neutral"} />
        <Tile label="File writes" value={fileWrites} accent="neutral" />
        <Tile label="Process execs" value={procExecs} accent="neutral" />
      </div>

      {blocked > 0 && (
        <div className="rounded-md border border-block-500/30 bg-block-50 p-3 text-sm text-block-700 flex items-start gap-2">
          <AlertTriangle className="h-4 w-4 mt-0.5" />
          <div>
            Citadel blocked <b className="mono">{blocked}</b> outbound connection{blocked > 1 ? "s" : ""} in this run.
            The packets were dropped at the kernel by the <span className="mono">cgroup_skb/egress</span> filter.
          </div>
        </div>
      )}

      <DetectionsList detections={detections} />
    </div>
  );
}

function Tile({ label, value, accent }: { label: string; value: number; accent: "brand" | "ok" | "block" | "neutral" }) {
  const accentCls = {
    brand:   "text-brand-700",
    ok:      "text-ok-700",
    block:   "text-block-700",
    neutral: "text-ink",
  }[accent];
  return (
    <div className="rounded-md border border-surface-line bg-surface-card shadow-card p-3">
      <div className="text-xs uppercase text-ink-muted tracking-wide">{label}</div>
      <div className={`mt-1 text-2xl font-semibold mono ${accentCls}`}>{value}</div>
    </div>
  );
}

function DetectionsList({ detections }: { detections: DetectionRow[] }) {
  return (
    <Card title="Detections" count={detections.length}>
      {detections.length === 0 ? (
        <p className="text-ink-subtle text-sm p-4">No findings.</p>
      ) : (
        <ul className="divide-y divide-surface-line">
          {detections.map((d) => (
            <li key={d.id} className="px-3 py-2.5 hover:bg-brand-50/40">
              <div className="flex items-center gap-2">
                <SeverityBadge severity={d.severity} />
                <span className="mono text-sm text-ink-muted">{d.rule_name}</span>
                <span className="ml-auto text-xs text-ink-subtle">{new Date(d.created_at).toLocaleTimeString()}</span>
              </div>
              <p className="mt-1 text-sm">{d.message}</p>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}

// ============================================================================
// Network Events tab — primary table, StepSecurity-style columns
// ============================================================================

function NetworkTab({ events }: { events: CitadelEvent[] }) {
  const [filter, setFilter] = useState("");
  const [showBlocked, setShowBlocked] = useState<"all" | "allowed" | "blocked">("all");

  const rows = events.filter((e) => {
    if (showBlocked === "allowed" && e.network?.blocked === true) return false;
    if (showBlocked === "blocked" && e.network?.blocked !== true) return false;
    if (!filter) return true;
    const s = filter.toLowerCase();
    return (
      (e.network?.hostname ?? "").toLowerCase().includes(s) ||
      (e.network?.dst_ip ?? "").toLowerCase().includes(s) ||
      (e.network?.process ?? "").toLowerCase().includes(s) ||
      (e.workflow.step ?? "").toLowerCase().includes(s)
    );
  });

  return (
    <Card
      title="Network Events"
      count={rows.length}
      toolbar={
        <div className="flex items-center gap-2">
          <input
            placeholder="filter…"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            className="rounded border border-surface-line px-2 py-1 text-sm mono focus:outline-none focus:border-brand-500"
          />
          <select
            value={showBlocked}
            onChange={(e) => setShowBlocked(e.target.value as any)}
            className="rounded border border-surface-line bg-surface-card px-2 py-1 text-sm"
          >
            <option value="all">All</option>
            <option value="allowed">Allowed only</option>
            <option value="blocked">Blocked only</option>
          </select>
        </div>
      }
    >
      <DenseTable
        head={["Status", "Time", "Process", "Destination", "Step", "Chain"]}
        rows={rows.map((e) => [
          <StatusPill key="s" allowed={e.network?.blocked !== true} />,
          <span className="mono text-ink-muted text-xs">{new Date(e.timestamp).toLocaleTimeString()}</span>,
          <span className="mono">{e.network?.process || "?"}</span>,
          <span className="mono">{e.network?.hostname || e.network?.dst_ip}<span className="text-ink-subtle">:{e.network?.dst_port}</span></span>,
          <span className="text-ink-muted">{e.workflow.step || "—"}</span>,
          <span className="mono text-xs text-ink-subtle">{(e.process_chain ?? []).join(" → ") || "—"}</span>,
        ])}
      />
    </Card>
  );
}

// ============================================================================
// File Write Events tab
// ============================================================================

function FilesTab({ events }: { events: CitadelEvent[] }) {
  const rows = events.map((e) => {
    const tampered = e.type === "file_tamper";
    return [
      <StatusPill key="s" allowed={!tampered} />,
      <span className="mono text-ink-muted text-xs">{new Date(e.timestamp).toLocaleTimeString()}</span>,
      <span className="mono">{e.process_chain?.[0] || "?"}</span>,
      <span className="mono break-all">{e.file?.path}</span>,
      <span className="mono text-xs">{tampered ? e.file?.action || "modified" : e.file?.flags}</span>,
      <span className="text-ink-muted">{e.workflow.step || "—"}</span>,
    ];
  });
  return (
    <Card title="File Write Events" count={rows.length}>
      <DenseTable
        head={["Status", "Time", "Process", "Path", "Flags / Action", "Step"]}
        rows={rows}
      />
    </Card>
  );
}

// ============================================================================
// Process Events tab — tree + flat table
// ============================================================================

function ProcessTab({ tree, flat }: { tree: ProcessTreeNode[] | null; flat: CitadelEvent[] }) {
  return (
    <div className="space-y-4">
      <Card title="Process tree" count={tree?.length ?? 0}>
        {tree && tree.length > 0 ? (
          <div className="p-3 space-y-1">
            {tree.map((n) => <TreeNode key={n.pid} node={n} depth={0} />)}
          </div>
        ) : (
          <p className="text-ink-subtle text-sm p-4">No process exec events captured.</p>
        )}
      </Card>
      <Card title="Process exec events" count={flat.length}>
        <DenseTable
          head={["Time", "PID", "PPID", "Comm", "Filename", "Args", "Step"]}
          rows={flat.map((e) => [
            <span className="mono text-ink-muted text-xs">{new Date(e.timestamp).toLocaleTimeString()}</span>,
            <span className="mono">{e.process?.pid}</span>,
            <span className="mono text-ink-muted">{e.process?.ppid}</span>,
            <span className="mono font-medium">{e.process?.comm}</span>,
            <span className="mono text-ink-muted">{e.process?.filename}</span>,
            <span className="mono text-ink-muted truncate inline-block max-w-[24rem]">{(e.process?.args ?? []).join(" ")}</span>,
            <span className="text-ink-muted">{e.workflow.step || "—"}</span>,
          ])}
        />
      </Card>
    </div>
  );
}

function TreeNode({ node, depth }: { node: ProcessTreeNode; depth: number }) {
  const [open, setOpen] = useState(true);
  return (
    <div style={{ paddingLeft: depth * 16 }}>
      <button
        onClick={() => setOpen(!open)}
        className="text-left text-sm w-full hover:bg-brand-50/40 rounded px-2 py-1 flex items-center gap-2"
      >
        <span className="text-ink-subtle mono">{node.children?.length ? (open ? "▾" : "▸") : "•"}</span>
        <span className="mono font-medium">{node.comm}</span>
        <span className="mono text-xs text-ink-subtle">pid={node.pid}</span>
        {node.args && node.args.length > 1 && (
          <span className="mono text-xs text-ink-muted truncate">{node.args.slice(1).join(" ")}</span>
        )}
      </button>
      {open && node.children?.map((c) => <TreeNode key={c.pid} node={c} depth={depth + 1} />)}
    </div>
  );
}

// ============================================================================
// Action Logs tab — GitHub Actions annotations/log lines
// ============================================================================

function ActionLogsTab({ logs }: { logs: GitHubActionLogRow[] }) {
  return (
    <Card title="GitHub Actions logs" count={logs.length}>
      {logs.length === 0 ? (
        <p className="text-ink-subtle text-sm p-4">No GitHub Actions log annotations captured for this run.</p>
      ) : (
        <ul className="divide-y divide-surface-line">
          {logs.map((log) => (
            <li key={log.id} className="px-3 py-3">
              <div className="flex items-center gap-2">
                <SeverityBadge severity={normalizeLogLevel(log.level)} />
                {log.rule_name && <span className="mono text-sm text-ink-muted">{log.rule_name}</span>}
                <span className="text-xs text-ink-subtle">{new Date(log.created_at).toLocaleTimeString()}</span>
                {log.html_url && (
                  <a
                    href={log.html_url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="ml-auto text-xs text-brand-600 hover:underline"
                  >
                    Open in GitHub
                  </a>
                )}
              </div>
              <p className="text-sm mt-1">{log.message}</p>
              <div className="mt-1 text-xs text-ink-muted">
                {[log.job_name, log.step, log.line ? `line ${log.line}` : ""].filter(Boolean).join(" · ") || "GitHub Actions"}
              </div>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}

function normalizeLogLevel(level: string): "info" | "low" | "medium" | "high" | "critical" {
  if (level === "critical" || level === "high" || level === "medium" || level === "low" || level === "info") {
    return level;
  }
  if (level === "error") return "high";
  if (level === "warning") return "medium";
  return "info";
}

// ============================================================================
// Shared primitives
// ============================================================================

function Card({ title, count, toolbar, children }: { title: string; count?: number; toolbar?: React.ReactNode; children: React.ReactNode }) {
  return (
    <div className="rounded-md border border-surface-line bg-surface-card shadow-card overflow-hidden">
      <div className="flex items-center justify-between border-b border-surface-line bg-surface-rail/60 px-3 py-2">
        <div className="text-sm font-medium text-ink flex items-center gap-2">
          {title}
          {count !== undefined && <span className="rounded-full bg-surface-line text-ink-muted px-1.5 text-xs mono">{count}</span>}
        </div>
        {toolbar}
      </div>
      {children}
    </div>
  );
}

function DenseTable({ head, rows }: { head: string[]; rows: React.ReactNode[][] }) {
  return (
    <table className="w-full text-sm">
      <thead className="bg-surface-rail text-ink-muted text-xs uppercase tracking-wide">
        <tr>
          {head.map((h) => <th key={h} className="px-3 py-2 text-left font-medium">{h}</th>)}
        </tr>
      </thead>
      <tbody className="divide-y divide-surface-line">
        {rows.length === 0 ? (
          <tr><td colSpan={head.length} className="px-3 py-6 text-ink-subtle text-center">No events.</td></tr>
        ) : (
          rows.map((r, i) => (
            <tr key={i} className="hover:bg-brand-50/40">
              {r.map((cell, j) => <td key={j} className="px-3 py-2 align-middle">{cell}</td>)}
            </tr>
          ))
        )}
      </tbody>
    </table>
  );
}

function ErrorBox({ msg }: { msg: string }) {
  return <div className="rounded border border-block-500/40 bg-block-50 p-3 mono text-block-700">{msg}</div>;
}
