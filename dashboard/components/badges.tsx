import type { Severity } from "@/lib/types";

const SEV_CLASSES: Record<Severity, string> = {
  info: "bg-slate-700/40 text-slate-300 border-slate-600",
  low: "bg-blue-900/40 text-blue-300 border-blue-700",
  medium: "bg-amber-900/40 text-amber-300 border-amber-700",
  high: "bg-orange-900/40 text-orange-300 border-orange-700",
  critical: "bg-red-900/40 text-red-300 border-red-700",
};

export function SeverityBadge({ severity }: { severity: Severity | string }) {
  const cls = SEV_CLASSES[severity as Severity] ?? SEV_CLASSES.info;
  return (
    <span
      className={`inline-flex items-center rounded border px-2 py-0.5 text-xs font-medium uppercase tracking-wide ${cls}`}
    >
      {severity}
    </span>
  );
}

export function ModeBadge({ mode }: { mode: string }) {
  const isBlock = mode === "block";
  const cls = isBlock
    ? "bg-red-900/40 text-red-300 border-red-700"
    : "bg-amber-900/40 text-amber-300 border-amber-700";
  return (
    <span className={`inline-flex items-center rounded border px-2 py-0.5 text-xs font-medium uppercase ${cls}`}>
      {mode}
    </span>
  );
}

/** A tiny number-with-icon group, e.g. for event-count summaries. */
export function CountChip({ icon, value, label }: { icon: React.ReactNode; value: number; label?: string }) {
  return (
    <span title={label} className="inline-flex items-center gap-1 text-xs text-slate-300">
      {icon}
      <span className="mono">{value}</span>
    </span>
  );
}
