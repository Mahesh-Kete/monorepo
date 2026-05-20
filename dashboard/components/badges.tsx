import { Check, X, AlertTriangle } from "lucide-react";
import type { Severity } from "@/lib/types";

// ---------------------------------------------------------------------------
// StatusPill — "Allowed" / "Blocked" column on Network + File tables.
// Uses CSS-variable-backed colors so it auto-flips in dark mode.
// ---------------------------------------------------------------------------
export function StatusPill({ allowed }: { allowed: boolean }) {
  if (allowed) {
    return (
      <span className="inline-flex items-center gap-1 rounded-full border border-ok-500/30 bg-ok-50 px-2 py-0.5 text-xs font-medium text-ok-700">
        <Check className="h-3 w-3" />
        Allowed
      </span>
    );
  }
  return (
    <span className="inline-flex items-center gap-1 rounded-full border border-block-500/30 bg-block-50 px-2 py-0.5 text-xs font-medium text-block-700">
      <X className="h-3 w-3" />
      Blocked
    </span>
  );
}

// ---------------------------------------------------------------------------
// SeverityBadge — info/low/medium/high/critical. medium + critical use the
// CSS-variable warn/block tokens (auto dark mode); info/low/high have
// explicit dark: variants.
// ---------------------------------------------------------------------------
const SEV_CLASSES: Record<Severity, string> = {
  info:     "bg-slate-100 text-slate-700 border-slate-300 dark:bg-slate-800 dark:text-slate-300 dark:border-slate-700",
  low:      "bg-sky-50 text-sky-700 border-sky-300 dark:bg-sky-900/30 dark:text-sky-300 dark:border-sky-700",
  medium:   "bg-warn-50 text-warn-700 border-warn-500/40",
  high:     "bg-orange-50 text-orange-700 border-orange-300 dark:bg-orange-900/30 dark:text-orange-300 dark:border-orange-700",
  critical: "bg-block-50 text-block-700 border-block-500/40",
};

export function SeverityBadge({ severity }: { severity: Severity | string }) {
  const cls = SEV_CLASSES[severity as Severity] ?? SEV_CLASSES.info;
  return (
    <span
      className={`inline-flex items-center gap-1 rounded border px-2 py-0.5 text-xs font-medium uppercase tracking-wide ${cls}`}
    >
      {(severity === "critical" || severity === "high") && (
        <AlertTriangle className="h-3 w-3" />
      )}
      {severity}
    </span>
  );
}

// ---------------------------------------------------------------------------
// ModeBadge — policy mode (audit / block) per run.
// ---------------------------------------------------------------------------
export function ModeBadge({ mode }: { mode: string }) {
  const normalized = mode.toLowerCase();
  const isBlock = normalized === "block";
  const cls = isBlock
    ? "bg-block-50 text-block-700 border-block-500/40"
    : "bg-warn-50 text-warn-700 border-warn-500/40";
  return (
    <span
      className={`inline-flex items-center rounded border px-2 py-0.5 text-xs font-medium uppercase ${cls}`}
    >
      {mode}
    </span>
  );
}

// ---------------------------------------------------------------------------
// RunStatusBadge — LeetCode-style status chip for workflow runs.
// ---------------------------------------------------------------------------
export function RunStatusBadge({ status }: { status: string }) {
  const normalized = normalizeRunStatus(status);
  const cls = {
    success: "border-ok-500/30 bg-ok-50 text-ok-700",
    progress: "border-brand-500/30 bg-brand-50 text-brand-700",
    failure: "border-block-500/30 bg-block-50 text-block-700",
    neutral: "border-surface-line bg-surface-rail text-ink-muted",
  }[normalized.kind];

  return (
    <span className={`inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-medium ${cls}`}>
      {normalized.label}
    </span>
  );
}

function normalizeRunStatus(status: string) {
  const s = status.toLowerCase();
  if (s === "success" || s === "succeeded" || s === "completed") {
    return { kind: "success" as const, label: "Success" };
  }
  if (s === "in_progress" || s === "running" || s === "queued" || s === "pending" || s === "requested" || s === "waiting") {
    return { kind: "progress" as const, label: "In Progress" };
  }
  if (s === "failure" || s === "failed" || s === "cancelled" || s === "timed_out" || s === "action_required") {
    return { kind: "failure" as const, label: "Failure" };
  }
  return {
    kind: "neutral" as const,
    label: status ? status.replace("_", " ") : "Unknown",
  };
}

// ---------------------------------------------------------------------------
// JobStatusDot — compact colored circle for places that still need an icon.
// ---------------------------------------------------------------------------
export function JobStatusDot({ status }: { status: string }) {
  const s = status.toLowerCase();
  let cls = "bg-slate-400 dark:bg-slate-500";
  if (s === "completed" || s === "success" || s === "succeeded") cls = "bg-ok-500";
  else if (s === "failure" || s === "failed" || s === "cancelled") cls = "bg-block-500";
  else if (s === "in_progress" || s === "running") cls = "bg-brand-500 animate-pulse";
  return <span className={`inline-block h-2 w-2 rounded-full ${cls}`} />;
}
