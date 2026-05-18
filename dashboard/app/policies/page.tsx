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
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-semibold tracking-tight flex items-center gap-2">
          <Lock className="h-5 w-5 text-cyan-400" /> Policies
        </h1>
        <button
          onClick={() => setShowNew(true)}
          className="inline-flex items-center gap-2 rounded border border-cyan-700 bg-cyan-900/30 px-3 py-1.5 text-sm text-cyan-300 hover:bg-cyan-900/50"
        >
          <Plus className="h-4 w-4" /> New Policy
        </button>
      </div>

      {err && <div className="rounded border border-red-800 bg-red-950/40 p-3 text-sm text-red-300 mono mb-4">{err}</div>}

      {policies === null ? (
        <div className="text-slate-400">Loading…</div>
      ) : policies.length === 0 ? (
        <div className="rounded-lg border border-slate-800 bg-slate-900/40 p-8 text-center text-slate-400">
          <Lock className="h-10 w-10 text-slate-700 mx-auto mb-3" />
          <p>No policies defined.</p>
          <p className="text-sm text-slate-500 mt-1">All runs use the permissive default until you create one.</p>
        </div>
      ) : (
        <div className="rounded-lg border border-slate-800 overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-slate-900/60 text-slate-400 text-xs uppercase tracking-wide">
              <tr>
                <th className="px-3 py-2 text-left">Name</th>
                <th className="px-3 py-2 text-left">Scope</th>
                <th className="px-3 py-2 text-left">Mode</th>
                <th className="px-3 py-2 text-left">Allowlist</th>
                <th className="px-3 py-2 text-left">Updated</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800">
              {policies.map((p) => (
                <tr key={p.id} className="hover:bg-slate-900/40">
                  <td className="px-3 py-2 mono font-semibold">{p.name}</td>
                  <td className="px-3 py-2 mono text-slate-300">
                    {p.scope_repo || "*"}
                    {p.scope_workflow ? ` · ${p.scope_workflow}` : ""}
                  </td>
                  <td className="px-3 py-2"><ModeBadge mode={p.mode} /></td>
                  <td className="px-3 py-2 mono text-slate-400">
                    {(p.allowlist || []).slice(0, 3).join(", ") || "—"}
                    {(p.allowlist?.length ?? 0) > 3 && ` +${(p.allowlist!.length - 3)}`}
                  </td>
                  <td className="px-3 py-2 text-slate-400">{p.updated_at ? new Date(p.updated_at).toLocaleString() : "—"}</td>
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
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-20">
      <div className="bg-slate-900 border border-slate-700 rounded-lg p-6 w-full max-w-lg">
        <h2 className="text-lg font-semibold mb-4">New Policy</h2>
        <div className="space-y-3 text-sm">
          <Field label="Name" value={name} setValue={setName} placeholder="block-victim-ci" />
          <Field label="Scope repo" value={repo} setValue={setRepo} placeholder="org/repo" />
          <Field label="Scope workflow" value={workflow} setValue={setWorkflow} placeholder="victim-ci" />
          <div>
            <label className="block text-slate-400 mb-1">Mode</label>
            <select
              value={mode}
              onChange={(e) => setMode(e.target.value as "audit" | "block")}
              className="w-full rounded border border-slate-700 bg-slate-950 px-3 py-1.5 mono"
            >
              <option value="audit">audit</option>
              <option value="block">block</option>
            </select>
          </div>
          <div>
            <label className="block text-slate-400 mb-1">Allowlist (one domain per line)</label>
            <textarea
              value={allowlist}
              onChange={(e) => setAllowlist(e.target.value)}
              rows={5}
              placeholder="github.com&#10;registry.npmjs.org"
              className="w-full rounded border border-slate-700 bg-slate-950 px-3 py-1.5 mono"
            />
          </div>
          {err && <div className="rounded border border-red-800 bg-red-950/40 p-2 mono text-red-300">{err}</div>}
        </div>
        <div className="flex justify-end gap-2 mt-5">
          <button onClick={onClose} className="rounded border border-slate-700 px-3 py-1.5 text-slate-300 hover:bg-slate-800">Cancel</button>
          <button onClick={save} className="rounded bg-cyan-600 px-3 py-1.5 text-white hover:bg-cyan-500">Save</button>
        </div>
      </div>
    </div>
  );
}

function Field({ label, value, setValue, placeholder }: { label: string; value: string; setValue: (s: string) => void; placeholder?: string }) {
  return (
    <div>
      <label className="block text-slate-400 mb-1">{label}</label>
      <input
        value={value}
        onChange={(e) => setValue(e.target.value)}
        placeholder={placeholder}
        className="w-full rounded border border-slate-700 bg-slate-950 px-3 py-1.5 mono"
      />
    </div>
  );
}
