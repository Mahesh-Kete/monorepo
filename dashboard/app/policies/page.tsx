"use client";

import { useEffect, useState } from "react";
import { Plus, Lock, Trash2, X, Shield } from "lucide-react";
import { api } from "@/lib/api";
import type { Policy } from "@/lib/types";

export default function PoliciesPage() {
  const [policies, setPolicies] = useState<Policy[] | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [showNew, setShowNew] = useState(false);
  const [openId, setOpenId] = useState<number | null>(null);

  const load = () => api.listPolicies().then(setPolicies).catch((e) => setErr(String(e)));
  useEffect(() => { load(); }, []);

  const onDelete = async (p: Policy) => {
    if (!confirm(`Delete policy "${p.name}"? This is permanent.`)) return;
    try {
      await api.deletePolicy(p.id);
      // If the deleted policy was being viewed, close the modal.
      setOpenId((cur) => (cur === p.id ? null : cur));
      await load();
    } catch (e) {
      alert(`Delete failed: ${(e as Error).message}`);
    }
  };

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-xl font-semibold tracking-tight flex items-center gap-2">
          <Lock className="h-5 w-5 text-brand-600" /> Policies
        </h1>
        <button
          onClick={() => setShowNew(true)}
          className="inline-flex items-center gap-1.5 rounded bg-brand-600 px-3 py-1.5 text-sm text-white hover:bg-brand-700"
        >
          <Plus className="h-4 w-4" /> New Policy
        </button>
      </div>

      {err && (
        <div className="rounded border border-block-500/40 bg-block-50 p-3 text-sm text-block-700 mono mb-4">
          {err}
        </div>
      )}

      {policies === null ? (
        <div className="text-ink-subtle">Loading…</div>
      ) : policies.length === 0 ? (
        <div className="rounded-md border border-surface-line bg-surface-card p-10 text-center">
          <Lock className="h-10 w-10 text-ink-subtle mx-auto mb-3" />
          <p className="font-medium">No policies defined.</p>
          <p className="text-sm text-ink-muted mt-1">
            All runs use the permissive default until you create one.
          </p>
        </div>
      ) : (
        <div className="rounded-md border border-surface-line bg-surface-card shadow-card overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-surface-rail text-ink-muted text-xs uppercase tracking-wide">
              <tr>
                <Th>Name</Th>
                <Th>Scope</Th>
                <Th>Allowlist</Th>
                <Th>Updated</Th>
                <Th className="w-12 text-right"></Th>
              </tr>
            </thead>
            <tbody className="divide-y divide-surface-line">
              {policies.map((p) => (
                <tr
                  key={p.id}
                  onClick={() => setOpenId(p.id)}
                  className="hover:bg-brand-50/40 cursor-pointer"
                >
                  <Td className="mono font-medium text-brand-700">{p.name}</Td>
                  <Td className="mono text-ink-muted">
                    {p.scope_repo || "*"}
                    {p.scope_workflow ? ` · ${p.scope_workflow}` : ""}
                  </Td>
                  <Td className="mono text-ink-muted truncate max-w-[24rem]">
                    {(p.allowlist || []).slice(0, 3).join(", ") || "—"}
                    {(p.allowlist?.length ?? 0) > 3 && ` +${(p.allowlist!.length - 3)}`}
                  </Td>
                  <Td className="text-ink-muted">
                    {p.updated_at ? new Date(p.updated_at).toLocaleString() : "—"}
                  </Td>
                  <Td className="text-right">
                    <button
                      onClick={(e) => { e.stopPropagation(); onDelete(p); }}
                      title="Delete policy"
                      aria-label="Delete policy"
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

      {showNew && <NewPolicyDialog onClose={() => setShowNew(false)} onSaved={() => { setShowNew(false); load(); }} />}
      {openId !== null && (
        <PolicyDetailDialog
          id={openId}
          onClose={() => setOpenId(null)}
          onDelete={async (p) => {
            await onDelete(p);
          }}
        />
      )}
    </div>
  );
}

function Th({ children, className = "" }: { children?: React.ReactNode; className?: string }) {
  return <th className={`px-3 py-2 text-left font-medium ${className}`}>{children}</th>;
}
function Td({ children, className = "" }: { children: React.ReactNode; className?: string }) {
  return <td className={`px-3 py-2 ${className}`}>{children}</td>;
}

// ============================================================================
// Detail modal — click any row to open
// ============================================================================

function PolicyDetailDialog({
  id, onClose, onDelete,
}: {
  id: number;
  onClose: () => void;
  onDelete: (p: Policy) => Promise<void>;
}) {
  const [policy, setPolicy] = useState<Policy | null>(null);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    api.getPolicy(id)
      .then((p) => { if (!cancelled) setPolicy(p); })
      .catch((e) => { if (!cancelled) setErr(String(e)); });
    return () => { cancelled = true; };
  }, [id]);

  return (
    <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-20" onClick={onClose}>
      <div
        className="bg-surface-card border border-surface-line rounded-md shadow-lg p-6 w-full max-w-2xl max-h-[90vh] overflow-y-auto"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-start justify-between mb-4">
          <div className="flex items-center gap-2">
            <Shield className="h-5 w-5 text-brand-600" />
            <h2 className="text-lg font-semibold">
              {policy ? policy.name : "Loading…"}
            </h2>
          </div>
          <button
            onClick={onClose}
            className="rounded p-1 text-ink-subtle hover:bg-surface-rail"
            aria-label="Close"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {err && (
          <div className="rounded border border-block-500/40 bg-block-50 p-2 mono text-block-700 text-xs mb-3">
            {err}
          </div>
        )}

        {!policy ? (
          <div className="text-ink-subtle text-sm">Loading policy…</div>
        ) : (
          <div className="space-y-4 text-sm">
            <KV label="Scope repo" mono value={policy.scope_repo || "*  (applies to all repos)"} />
            <KV label="Scope workflow" mono value={policy.scope_workflow || "*  (applies to all workflows in scope)"} />
            <KV label="Last updated" value={policy.updated_at ? new Date(policy.updated_at).toLocaleString() : "—"} />

            <div>
              <div className="text-xs uppercase tracking-wide text-ink-muted mb-1">
                Allowlist ({(policy.allowlist ?? []).length})
              </div>
              {(policy.allowlist ?? []).length === 0 ? (
                <div className="text-ink-subtle text-xs italic">Empty — all destinations blocked</div>
              ) : (
                <ul className="rounded border border-surface-line bg-surface-rail/40 p-2 mono text-xs space-y-0.5 max-h-48 overflow-y-auto">
                  {(policy.allowlist ?? []).map((d, i) => (
                    <li key={i}>{d}</li>
                  ))}
                </ul>
              )}
            </div>

            <div>
              <div className="text-xs uppercase tracking-wide text-ink-muted mb-1">
                Detection rules
              </div>
              <pre className="rounded border border-surface-line bg-surface-rail/40 p-3 mono text-xs whitespace-pre-wrap overflow-x-auto max-h-72">
                {policy.detection_rules
                  ? JSON.stringify(policy.detection_rules, null, 2)
                  : "{}"}
              </pre>
            </div>
          </div>
        )}

        <div className="flex justify-between items-center mt-5 pt-4 border-t border-surface-line">
          {policy && (
            <button
              onClick={() => onDelete(policy)}
              className="inline-flex items-center gap-1.5 rounded border border-block-500/40 bg-block-50 px-3 py-1.5 text-sm text-block-700 hover:bg-block-100"
            >
              <Trash2 className="h-4 w-4" />
              Delete policy
            </button>
          )}
          <button
            onClick={onClose}
            className="rounded border border-surface-line bg-surface-card px-3 py-1.5 text-sm hover:bg-surface-rail ml-auto"
          >
            Close
          </button>
        </div>
      </div>
    </div>
  );
}

function KV({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <div>
      <div className="text-xs uppercase tracking-wide text-ink-muted mb-0.5">{label}</div>
      <div className={mono ? "mono" : ""}>{value}</div>
    </div>
  );
}

// ============================================================================
// New-policy dialog
// ============================================================================

function NewPolicyDialog({ onClose, onSaved }: { onClose: () => void; onSaved: () => void }) {
  const [name, setName] = useState("");
  const [repo, setRepo] = useState("");
  const [workflow, setWorkflow] = useState("");
  const [allowlist, setAllowlist] = useState("");
  const [err, setErr] = useState<string | null>(null);

  const save = async () => {
    try {
      const list = allowlist.split("\n").map((s) => s.trim()).filter(Boolean);
      await api.createPolicy({
        name,
        scope_repo: repo || undefined,
        scope_workflow: workflow || undefined,
        mode: "block",
        allowlist: list,
        detection_rules: {},
      });
      onSaved();
    } catch (e) {
      setErr(String(e));
    }
  };

  return (
    <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-20" onClick={onClose}>
      <div
        className="bg-surface-card border border-surface-line rounded-md shadow-lg p-6 w-full max-w-lg"
        onClick={(e) => e.stopPropagation()}
      >
        <h2 className="text-lg font-semibold mb-4">New Policy</h2>
        <p className="text-xs text-ink-muted mb-4">
          Citadel only has one mode now (<span className="mono">block</span>). The allowlist
          defines which destinations are permitted; everything else is dropped.
        </p>
        <div className="space-y-3 text-sm">
          <Field label="Name" value={name} setValue={setName} placeholder="victim-ci-block" />
          <Field label="Scope repo" value={repo} setValue={setRepo} placeholder="org/repo  (leave blank to apply to all)" />
          <Field label="Scope workflow" value={workflow} setValue={setWorkflow} placeholder="victim-ci  (leave blank for all workflows in scope)" />
          <div>
            <label className="block text-ink-muted mb-1">Allowlist (one domain per line)</label>
            <textarea
              value={allowlist}
              onChange={(e) => setAllowlist(e.target.value)}
              rows={5}
              placeholder={"github.com\nregistry.npmjs.org"}
              className="w-full rounded border border-surface-line bg-surface-card px-3 py-1.5 mono"
            />
          </div>
          {err && (
            <div className="rounded border border-block-500/40 bg-block-50 p-2 mono text-block-700">{err}</div>
          )}
        </div>
        <div className="flex justify-end gap-2 mt-5">
          <button onClick={onClose} className="rounded border border-surface-line bg-surface-card px-3 py-1.5 text-ink hover:bg-surface-rail">Cancel</button>
          <button onClick={save} disabled={!name} className="rounded bg-brand-600 px-3 py-1.5 text-white hover:bg-brand-700 disabled:opacity-50">Save</button>
        </div>
      </div>
    </div>
  );
}

function Field({ label, value, setValue, placeholder }: { label: string; value: string; setValue: (s: string) => void; placeholder?: string }) {
  return (
    <div>
      <label className="block text-ink-muted mb-1">{label}</label>
      <input
        value={value}
        onChange={(e) => setValue(e.target.value)}
        placeholder={placeholder}
        className="w-full rounded border border-surface-line bg-surface-card px-3 py-1.5 mono"
      />
    </div>
  );
}
