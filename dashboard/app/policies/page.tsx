"use client";

import { useEffect, useState } from "react";
import { Plus, Lock } from "lucide-react";
import { api } from "@/lib/api";
import type { Policy } from "@/lib/types";
import { ModeBadge } from "@/components/badges";

export default function PoliciesPage() {
  const [policies, setPolicies] = useState<Policy[] | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [showNew, setShowNew] = useState(false);

  const load = () => api.listPolicies().then(setPolicies).catch((e) => setErr(String(e)));
  useEffect(() => { load(); }, []);

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

      {err && <div className="rounded border border-block-500/40 bg-block-50 p-3 text-sm text-block-700 mono mb-4">{err}</div>}

      {policies === null ? (
        <div className="text-ink-subtle">Loading…</div>
      ) : policies.length === 0 ? (
        <div className="rounded-md border border-surface-line bg-surface-card p-10 text-center">
          <Lock className="h-10 w-10 text-ink-subtle mx-auto mb-3" />
          <p className="font-medium">No policies defined.</p>
          <p className="text-sm text-ink-muted mt-1">All runs use the permissive default until you create one.</p>
        </div>
      ) : (
        <div className="rounded-md border border-surface-line bg-surface-card shadow-card overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-surface-rail text-ink-muted text-xs uppercase tracking-wide">
              <tr>
                <Th>Name</Th>
                <Th>Scope</Th>
                <Th>Mode</Th>
                <Th>Allowlist</Th>
                <Th>Updated</Th>
              </tr>
            </thead>
            <tbody className="divide-y divide-surface-line">
              {policies.map((p) => (
                <tr key={p.id} className="hover:bg-brand-50/40">
                  <Td className="mono font-medium">{p.name}</Td>
                  <Td className="mono text-ink-muted">
                    {p.scope_repo || "*"}
                    {p.scope_workflow ? ` · ${p.scope_workflow}` : ""}
                  </Td>
                  <Td><ModeBadge mode={p.mode} /></Td>
                  <Td className="mono text-ink-muted">
                    {(p.allowlist || []).slice(0, 3).join(", ") || "—"}
                    {(p.allowlist?.length ?? 0) > 3 && ` +${(p.allowlist!.length - 3)}`}
                  </Td>
                  <Td className="text-ink-muted">{p.updated_at ? new Date(p.updated_at).toLocaleString() : "—"}</Td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {showNew && <NewPolicyDialog onClose={() => setShowNew(false)} onSaved={() => { setShowNew(false); load(); }} />}
    </div>
  );
}

function Th({ children }: { children: React.ReactNode }) {
  return <th className="px-3 py-2 text-left font-medium">{children}</th>;
}
function Td({ children, className = "" }: { children: React.ReactNode; className?: string }) {
  return <td className={`px-3 py-2 ${className}`}>{children}</td>;
}

function NewPolicyDialog({ onClose, onSaved }: { onClose: () => void; onSaved: () => void }) {
  const [name, setName] = useState("");
  const [repo, setRepo] = useState("");
  const [workflow, setWorkflow] = useState("");
  const [mode, setMode] = useState<"audit" | "block">("audit");
  const [allowlist, setAllowlist] = useState("");
  const [err, setErr] = useState<string | null>(null);

  const save = async () => {
    try {
      const list = allowlist.split("\n").map((s) => s.trim()).filter(Boolean);
      await api.createPolicy({
        name,
        scope_repo: repo || undefined,
        scope_workflow: workflow || undefined,
        mode,
        allowlist: list,
        detection_rules: {},
      });
      onSaved();
    } catch (e) {
      setErr(String(e));
    }
  };

  return (
    <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-20">
      <div className="bg-surface-card border border-surface-line rounded-md shadow-lg p-6 w-full max-w-lg">
        <h2 className="text-lg font-semibold mb-4">New Policy</h2>
        <div className="space-y-3 text-sm">
          <Field label="Name" value={name} setValue={setName} placeholder="victim-ci-block" />
          <Field label="Scope repo" value={repo} setValue={setRepo} placeholder="org/repo" />
          <Field label="Scope workflow" value={workflow} setValue={setWorkflow} placeholder="victim-ci" />
          <div>
            <label className="block text-ink-muted mb-1">Mode</label>
            <select
              value={mode}
              onChange={(e) => setMode(e.target.value as "audit" | "block")}
              className="w-full rounded border border-surface-line bg-surface-card px-3 py-1.5 mono"
            >
              <option value="audit">audit</option>
              <option value="block">block</option>
            </select>
          </div>
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
          {err && <div className="rounded border border-block-500/40 bg-block-50 p-2 mono text-block-700">{err}</div>}
        </div>
        <div className="flex justify-end gap-2 mt-5">
          <button onClick={onClose} className="rounded border border-surface-line bg-surface-card px-3 py-1.5 text-ink hover:bg-surface-rail">Cancel</button>
          <button onClick={save} className="rounded bg-brand-600 px-3 py-1.5 text-white hover:bg-brand-700">Save</button>
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
