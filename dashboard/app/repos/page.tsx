"use client";

import { useState } from "react";
import { Plus, FolderGit2, RefreshCw, Trash2, AlertTriangle, CheckCircle2 } from "lucide-react";
import { api } from "@/lib/api";
import type { ConnectedRepo } from "@/lib/types";
import { LiveIndicator } from "@/components/live-indicator";
import { useLivePoll } from "@/lib/use-live-poll";

export default function ReposPage() {
  const { data: repos, error, updatedAt, refetch } = useLivePoll<ConnectedRepo[]>(
    () => api.listRepos(),
    5000,
  );
  const [showConnect, setShowConnect] = useState(false);

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-baseline gap-3">
          <h1 className="text-xl font-semibold tracking-tight flex items-center gap-2">
            <FolderGit2 className="h-5 w-5 text-brand-600" /> Connected Repositories
          </h1>
          <LiveIndicator updatedAt={updatedAt} />
        </div>
        <button
          onClick={() => setShowConnect(true)}
          className="inline-flex items-center gap-1.5 rounded bg-brand-600 px-3 py-1.5 text-sm text-white hover:bg-brand-700"
        >
          <Plus className="h-4 w-4" /> Connect repository
        </button>
      </div>

      <p className="text-sm text-ink-muted mb-4">
        Citadel will poll each connected repo's GitHub Actions runs every 30 seconds and surface them on{" "}
        <a href="/runs" className="text-brand-600 hover:underline">/runs</a>. Runs that also have the{" "}
        <code className="mono">citadel-setup</code> step report runtime events; others show GitHub status only.
      </p>

      {error && (
        <div className="rounded border border-block-500/40 bg-block-50 p-3 text-sm text-block-700 mono mb-4">
          {error}
        </div>
      )}

      {repos === null ? (
        <div className="text-ink-subtle">Loading…</div>
      ) : repos.length === 0 ? (
        <EmptyState onAdd={() => setShowConnect(true)} />
      ) : (
        <div className="rounded-md border border-surface-line bg-surface-card shadow-card overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-surface-rail text-ink-muted text-xs uppercase tracking-wide">
              <tr>
                <Th>Repository</Th>
                <Th>Status</Th>
                <Th>Last polled</Th>
                <Th>Connected</Th>
                <Th className="text-right">Actions</Th>
              </tr>
            </thead>
            <tbody className="divide-y divide-surface-line">
              {repos.map((r) => (
                <RepoRow key={r.id} repo={r} onChanged={refetch} />
              ))}
            </tbody>
          </table>
        </div>
      )}

      {showConnect && (
        <ConnectDialog
          onClose={() => setShowConnect(false)}
          onSaved={() => { setShowConnect(false); refetch(); }}
        />
      )}
    </div>
  );
}

function Th({ children, className = "" }: { children: React.ReactNode; className?: string }) {
  return <th className={`px-3 py-2 text-left font-medium ${className}`}>{children}</th>;
}

function RepoRow({ repo, onChanged }: { repo: ConnectedRepo; onChanged: () => void }) {
  const [busy, setBusy] = useState(false);

  const refresh = async () => {
    setBusy(true);
    try { await api.refreshRepo(repo.id); await onChanged(); }
    finally { setBusy(false); }
  };
  const disconnect = async () => {
    if (!confirm(`Disconnect ${repo.repository}? Existing runs stay in the dashboard but won't refresh from GitHub.`)) return;
    setBusy(true);
    try { await api.deleteRepo(repo.id); await onChanged(); }
    finally { setBusy(false); }
  };

  const hasError = repo.last_error && repo.last_error.length > 0;

  return (
    <tr className="hover:bg-brand-50/40">
      <td className="px-3 py-2">
        <a
          href={`https://github.com/${repo.repository}`}
          target="_blank" rel="noopener noreferrer"
          className="font-medium text-brand-600 hover:underline mono"
        >
          {repo.repository}
        </a>
        {repo.note && <div className="text-xs text-ink-muted mt-0.5">{repo.note}</div>}
      </td>
      <td className="px-3 py-2">
        {hasError ? (
          <span className="inline-flex items-center gap-1.5 text-xs text-block-700">
            <AlertTriangle className="h-3.5 w-3.5" />
            <span className="mono">{repo.last_error}</span>
          </span>
        ) : (
          <span className="inline-flex items-center gap-1.5 text-xs text-ok-700">
            <CheckCircle2 className="h-3.5 w-3.5" />
            healthy
          </span>
        )}
      </td>
      <td className="px-3 py-2 text-ink-muted">
        {repo.last_polled_at ? new Date(repo.last_polled_at).toLocaleString() : "—"}
      </td>
      <td className="px-3 py-2 text-ink-muted">
        {new Date(repo.created_at).toLocaleDateString()}
      </td>
      <td className="px-3 py-2 text-right space-x-2">
        <button
          onClick={refresh}
          disabled={busy}
          title="Trigger an immediate poll"
          className="inline-flex items-center rounded border border-surface-line bg-surface-card px-2 py-1 text-xs hover:bg-brand-50 disabled:opacity-50"
        >
          <RefreshCw className={`h-3.5 w-3.5 ${busy ? "animate-spin" : ""}`} />
        </button>
        <button
          onClick={disconnect}
          disabled={busy}
          title="Disconnect"
          className="inline-flex items-center rounded border border-block-500/40 bg-block-50 px-2 py-1 text-xs text-block-700 hover:bg-block-500/20 disabled:opacity-50"
        >
          <Trash2 className="h-3.5 w-3.5" />
        </button>
      </td>
    </tr>
  );
}

function EmptyState({ onAdd }: { onAdd: () => void }) {
  return (
    <div className="rounded-md border border-surface-line bg-surface-card p-10 text-center">
      <FolderGit2 className="h-10 w-10 text-ink-subtle mx-auto mb-3" />
      <p className="font-medium">No repositories connected yet.</p>
      <p className="text-sm text-ink-muted mt-1 mb-4">
        Connect a GitHub repo to monitor its CI/CD runs from Citadel.
      </p>
      <button onClick={onAdd} className="rounded bg-brand-600 px-3 py-1.5 text-sm text-white hover:bg-brand-700">
        Connect your first repository
      </button>
    </div>
  );
}

function ConnectDialog({ onClose, onSaved }: { onClose: () => void; onSaved: () => void }) {
  const [repo, setRepo] = useState("");
  const [token, setToken] = useState("");
  const [note, setNote] = useState("");
  const [err, setErr] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [okMsg, setOkMsg] = useState<string | null>(null);

  const submit = async () => {
    setErr(null); setOkMsg(null); setSubmitting(true);
    try {
      const res = await api.connectRepo({ repository: repo.trim(), token: token.trim(), note: note.trim() || undefined });
      setOkMsg(`Connected as ${res.authenticated_as}. First poll will run within 30 s.`);
      setTimeout(onSaved, 700);
    } catch (e) {
      setErr(String(e));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-20" onClick={onClose}>
      <div className="bg-surface-card border border-surface-line rounded-md shadow-lg p-6 w-full max-w-lg" onClick={(e) => e.stopPropagation()}>
        <h2 className="text-lg font-semibold mb-1">Connect a GitHub repository</h2>
        <p className="text-sm text-ink-muted mb-4">
          Paste a Personal Access Token with <code className="mono">repo</code> scope. Citadel uses it only to read{" "}
          <code className="mono">/actions/runs</code>; it never stores the token in plaintext outside this server.
        </p>

        <div className="space-y-3 text-sm">
          <Field
            label="Repository"
            value={repo}
            setValue={setRepo}
            placeholder="owner/repo  (e.g. Mahesh-Kete/citadel)"
          />
          <Field
            label="Personal Access Token"
            value={token}
            setValue={setToken}
            placeholder="ghp_… (paste your fine-grained or classic PAT)"
            type="password"
          />
          <Field
            label="Note (optional)"
            value={note}
            setValue={setNote}
            placeholder="What is this repo? Visible only to you."
          />
          <p className="text-xs text-ink-subtle">
            <a
              href="https://github.com/settings/tokens/new?scopes=repo&description=Citadel"
              target="_blank" rel="noopener noreferrer"
              className="text-brand-600 hover:underline"
            >
              Create a token with the right scopes →
            </a>
          </p>
          {err && (
            <div className="rounded border border-block-500/40 bg-block-50 p-2 mono text-block-700 text-xs">
              {err}
            </div>
          )}
          {okMsg && (
            <div className="rounded border border-ok-500/40 bg-ok-50 p-2 text-ok-700 text-xs">
              {okMsg}
            </div>
          )}
        </div>

        <div className="flex justify-end gap-2 mt-5">
          <button
            onClick={onClose}
            className="rounded border border-surface-line bg-surface-card px-3 py-1.5 text-ink hover:bg-surface-rail"
          >
            Cancel
          </button>
          <button
            onClick={submit}
            disabled={submitting || !repo || !token}
            className="rounded bg-brand-600 px-3 py-1.5 text-white hover:bg-brand-700 disabled:opacity-50"
          >
            {submitting ? "Connecting…" : "Connect"}
          </button>
        </div>
      </div>
    </div>
  );
}

function Field({
  label, value, setValue, placeholder, type = "text",
}: {
  label: string; value: string; setValue: (s: string) => void; placeholder?: string; type?: string;
}) {
  return (
    <div>
      <label className="block text-ink-muted mb-1">{label}</label>
      <input
        type={type}
        value={value}
        onChange={(e) => setValue(e.target.value)}
        placeholder={placeholder}
        className="w-full rounded border border-surface-line bg-surface-card px-3 py-1.5 mono focus:outline-none focus:border-brand-500"
        autoComplete="off"
        spellCheck={false}
      />
    </div>
  );
}
