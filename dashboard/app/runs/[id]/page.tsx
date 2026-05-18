"use client";

import { useEffect, useMemo, useState } from "react";
import { useParams } from "next/navigation";
import { Globe2, Cpu, FileText, Clock, ShieldAlert, GitCommit } from "lucide-react";
import { api, type ProcessTreeNode } from "@/lib/api";
import type { CitadelEvent, DetectionRow, RunDetail } from "@/lib/types";
import { ModeBadge, SeverityBadge } from "@/components/badges";

type Tab = "network" | "process" | "file" | "timeline";

export default function RunDetailPage() {
  const params = useParams<{ id: string }>();
  const runId = Number(params.id);

  const [data, setData] = useState<RunDetail | null>(null);
  const [tree, setTree] = useState<ProcessTreeNode[] | null>(null);
  const [tab, setTab] = useState<Tab>("network");
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    if (!runId) return;
    Promise.all([api.getRun(runId), api.getProcessTree(runId)])
      .then(([d, t]) => {
        setData(d);
        setTree(t);
      })
      .catch((e) => setErr(String(e)));
  }, [runId]);

  const blockedCount = useMemo(() => {
    // Phase 8 will surface real blocked counts. For now we look for any event
    // whose payload contains "blocked":true under network — a forward-compat
    // placeholder.
    if (!data) return 0;
    return data.events.filter(
      (e) => e.type === "network" && (e as any).network?.blocked === true,
    ).length;
  }, [data]);

  if (err) {
    return <div className="rounded border border-red-800 bg-red-950/40 p-3 mono text-red-300">{err}</div>;
  }
  if (!data) return <div className="text-slate-400">Loading…</div>;

  const { run, events, detections } = data;
  const byType = (t: string) => events.filter((e) => e.type === t);

  return (
    <div className="grid grid-cols-12 gap-6">
      <div className="col-span-12 lg:col-span-9">
        <HeaderCard run={run} />

        {blockedCount > 0 && (
          <div className="rounded-lg border border-red-800 bg-red-950/40 p-3 mt-4 text-sm text-red-300">
            🛡️ Citadel blocked {blockedCount} outbound connection{blockedCount > 1 ? "s" : ""} in this run.
          </div>
        )}

        <div className="mt-6 border-b border-slate-800 flex gap-1">
          <TabButton t="network" cur={tab} set={setTab} icon={<Globe2 className="h-4 w-4" />} count={run.event_counts.network} />
          <TabButton t="process" cur={tab} set={setTab} icon={<Cpu className="h-4 w-4" />} count={run.event_counts.process} />
          <TabButton t="file"    cur={tab} set={setTab} icon={<FileText className="h-4 w-4" />} count={run.event_counts.file + run.event_counts.file_tamper} />
          <TabButton t="timeline" cur={tab} set={setTab} icon={<Clock className="h-4 w-4" />} count={events.length} />
        </div>

        <div className="mt-4">
          {tab === "network" && <NetworkTab events={byType("network")} />}
          {tab === "process" && <ProcessTab tree={tree} flat={byType("process")} />}
          {tab === "file" && <FileTab events={[...byType("file"), ...byType("file_tamper")]} />}
          {tab === "timeline" && <TimelineTab events={events} />}
        </div>
      </div>

      <aside className="col-span-12 lg:col-span-3">
        <DetectionsPanel detections={detections} />
      </aside>
    </div>
  );
}

function HeaderCard({ run }: { run: RunDetail["run"] }) {
  return (
    <div className="rounded-lg border border-slate-800 bg-slate-900/40 p-4">
      <div className="flex flex-wrap items-baseline gap-x-4 gap-y-1">
        <h1 className="text-xl font-semibold">{run.repository}</h1>
        {run.workflow && (
          <span className="text-slate-400">
            {run.workflow}
            {run.run_number ? ` #${run.run_number}` : ` (run ${run.run_id})`}
          </span>
        )}
        <ModeBadge mode={run.policy_mode} />
      </div>
      <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-sm text-slate-400">
        {run.sha && (
          <span className="inline-flex items-center gap-1 mono">
            <GitCommit className="h-3.5 w-3.5" /> {run.sha.slice(0, 12)}
          </span>
        )}
        {run.actor && <span>by {run.actor}</span>}
        {run.ref && <span className="mono text-slate-500">{run.ref}</span>}
        <span className="mono text-slate-500">started {new Date(run.started_at).toLocaleString()}</span>
      </div>
    </div>
  );
}

function TabButton({
  t, cur, set, icon, count,
}: { t: Tab; cur: Tab; set: (t: Tab) => void; icon: React.ReactNode; count: number }) {
  const active = t === cur;
  return (
    <button
      onClick={() => set(t)}
      className={
        "flex items-center gap-2 px-3 py-2 text-sm border-b-2 -mb-px transition-colors " +
        (active
          ? "border-cyan-400 text-cyan-400"
          : "border-transparent text-slate-300 hover:text-cyan-300")
      }
    >
      {icon}
      <span className="capitalize">{t}</span>
      <span className="rounded-full bg-slate-800 px-1.5 py-0.5 text-xs mono">{count}</span>
    </button>
  );
}

function NetworkTab({ events }: { events: CitadelEvent[] }) {
  const [filter, setFilter] = useState("");
  const rows = events.filter((e) => {
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
    <div>
      <input
        type="text"
        placeholder="filter by hostname, ip, process, step…"
        value={filter}
        onChange={(e) => setFilter(e.target.value)}
        className="w-full max-w-md rounded border border-slate-700 bg-slate-900 px-3 py-1.5 text-sm mono mb-3"
      />
      <Table
        head={["Time", "Host / IP", "Port", "Process", "Step", "Chain"]}
        rows={rows.map((e) => [
          new Date(e.timestamp).toLocaleTimeString(),
          e.network?.hostname || e.network?.dst_ip,
          <span className="mono">{e.network?.dst_port}</span>,
          <span className="mono">{e.network?.process || "—"}</span>,
          <span className="text-slate-400">{e.workflow.step || "—"}</span>,
          <span className="mono text-slate-500">{(e.process_chain ?? []).join(" → ") || "—"}</span>,
        ])}
      />
    </div>
  );
}

function ProcessTab({ tree, flat }: { tree: ProcessTreeNode[] | null; flat: CitadelEvent[] }) {
  if (tree && tree.length > 0) {
    return (
      <div className="space-y-1">
        {tree.map((n) => (
          <TreeNode key={n.pid} node={n} depth={0} />
        ))}
      </div>
    );
  }
  // Fallback: flat list of process events
  return (
    <Table
      head={["Time", "PID", "PPID", "Comm", "Filename", "Args"]}
      rows={flat.map((e) => [
        new Date(e.timestamp).toLocaleTimeString(),
        <span className="mono">{e.process?.pid}</span>,
        <span className="mono text-slate-500">{e.process?.ppid}</span>,
        <span className="mono font-semibold">{e.process?.comm}</span>,
        <span className="mono text-slate-400">{e.process?.filename}</span>,
        <span className="mono text-slate-400 truncate inline-block max-w-[28rem]">{(e.process?.args ?? []).join(" ")}</span>,
      ])}
    />
  );
}

function TreeNode({ node, depth }: { node: ProcessTreeNode; depth: number }) {
  const [open, setOpen] = useState(true);
  return (
    <div style={{ paddingLeft: depth * 16 }}>
      <button
        onClick={() => setOpen(!open)}
        className="text-left text-sm w-full hover:bg-slate-900/40 rounded px-2 py-1 flex items-center gap-2"
      >
        <span className="text-slate-500 mono">{node.children?.length ? (open ? "▾" : "▸") : "•"}</span>
        <span className="mono font-semibold">{node.comm}</span>
        <span className="mono text-xs text-slate-500">pid={node.pid}</span>
        {node.args && node.args.length > 0 && (
          <span className="mono text-xs text-slate-400 truncate">{node.args.slice(1).join(" ")}</span>
        )}
      </button>
      {open && node.children?.map((c) => <TreeNode key={c.pid} node={c} depth={depth + 1} />)}
    </div>
  );
}

function FileTab({ events }: { events: CitadelEvent[] }) {
  return (
    <Table
      head={["Time", "Path", "Flags / Action", "Type"]}
      rows={events.map((e) => {
        const tampered = e.type === "file_tamper";
        const path = e.file?.path ?? "";
        const workspace = path.includes("/home/runner/work") || path.includes("/_work/");
        return [
          new Date(e.timestamp).toLocaleTimeString(),
          <span className={`mono ${workspace ? "text-cyan-300" : ""}`}>{path}</span>,
          <span className="mono">{tampered ? e.file?.action ?? "modified" : e.file?.flags}</span>,
          <span className={tampered ? "text-red-400" : "text-slate-400"}>{e.type}</span>,
        ];
      })}
    />
  );
}

function TimelineTab({ events }: { events: CitadelEvent[] }) {
  const sorted = [...events].sort((a, b) => a.timestamp.localeCompare(b.timestamp));
  return (
    <div className="space-y-1">
      {sorted.map((e, i) => (
        <div key={i} className="flex items-center gap-3 text-sm font-mono">
          <span className="text-slate-500">{new Date(e.timestamp).toLocaleTimeString()}</span>
          <TypeIcon type={e.type} />
          <span className="text-slate-400">{e.workflow.step || "—"}</span>
          <span>{describe(e)}</span>
        </div>
      ))}
    </div>
  );
}

function TypeIcon({ type }: { type: string }) {
  if (type === "network") return <Globe2 className="h-3.5 w-3.5 text-cyan-400" />;
  if (type === "process") return <Cpu className="h-3.5 w-3.5 text-emerald-400" />;
  if (type === "file_tamper") return <ShieldAlert className="h-3.5 w-3.5 text-red-400" />;
  return <FileText className="h-3.5 w-3.5 text-amber-400" />;
}

function describe(e: CitadelEvent): string {
  if (e.type === "network" && e.network) {
    return `${e.network.process || "?"} → ${e.network.hostname || e.network.dst_ip}:${e.network.dst_port}`;
  }
  if (e.type === "process" && e.process) {
    return `${e.process.comm} ${(e.process.args || []).slice(1).join(" ")}`;
  }
  if ((e.type === "file" || e.type === "file_tamper") && e.file) {
    return `${e.file.action ?? e.file.flags ?? "write"} ${e.file.path}`;
  }
  return e.type;
}

function DetectionsPanel({ detections }: { detections: DetectionRow[] }) {
  return (
    <div className="rounded-lg border border-slate-800 bg-slate-900/40 p-3 sticky top-20">
      <h2 className="text-sm font-semibold mb-3 flex items-center gap-2">
        <ShieldAlert className="h-4 w-4 text-red-400" /> Detections
        <span className="ml-auto text-slate-500 mono">{detections.length}</span>
      </h2>
      {detections.length === 0 ? (
        <p className="text-sm text-slate-500">No findings.</p>
      ) : (
        <ul className="space-y-3">
          {detections.map((d) => (
            <li key={d.id} className="rounded border border-slate-800 bg-slate-950/40 p-2">
              <div className="flex items-center gap-2">
                <SeverityBadge severity={d.severity} />
                <span className="text-xs mono text-slate-400">{d.rule_name}</span>
              </div>
              <p className="mt-1 text-sm text-slate-200">{d.message}</p>
              <p className="mt-1 text-xs text-slate-500">{new Date(d.created_at).toLocaleString()}</p>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function Table({ head, rows }: { head: string[]; rows: React.ReactNode[][] }) {
  return (
    <div className="rounded-lg border border-slate-800 overflow-hidden">
      <table className="w-full text-sm">
        <thead className="bg-slate-900/60 text-slate-400 text-xs uppercase tracking-wide">
          <tr>
            {head.map((h) => (
              <th key={h} className="px-3 py-2 text-left">{h}</th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-800">
          {rows.length === 0 ? (
            <tr><td colSpan={head.length} className="px-3 py-4 text-slate-500 text-center">No events.</td></tr>
          ) : (
            rows.map((r, i) => (
              <tr key={i} className="hover:bg-slate-900/40">
                {r.map((cell, j) => <td key={j} className="px-3 py-2 align-top">{cell}</td>)}
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  );
}
