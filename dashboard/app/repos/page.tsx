"use client";

import { useState } from "react";
import Link from "next/link";
import { Plus, FolderGit2, RefreshCw, Trash2, AlertTriangle, CheckCircle2, ExternalLink } from "lucide-react";
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
    if (!confirm(
      `Disconnect ${repo.repository}?\n\n` +
      `This permanently deletes ALL of its runs, events, and detections ` +
      `from the dashboard. The repo on GitHub is unaffected.`
    )) return;
    setBusy(true);
    try { await api.deleteRepo(repo.id); await onChanged(); }
    finally { setBusy(false); }
  };

  const hasError = repo.last_error && repo.last_error.length > 0;

  return (
    <tr className="hover:bg-brand-50/40">
      <td className="px-3 py-2">
        <div className="flex items-center gap-2">
          <Link
            href={`/repos/${repo.id}`}
            className="font-medium text-brand-600 hover:underline mono"
          >
            {repo.repository}
          </Link>
          <a
            href={`https://github.com/${repo.repository}`}
            target="_blank" rel="noopener noreferrer"
            className="text-ink-subtle hover:text-brand-600"
            title="Open on GitHub"
          >
            <ExternalLink className="h-3.5 w-3.5" />
          </a>
        </div>
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

type ConnectResult = {
  authenticated_as: string;
  workflow_injected: boolean;
  workflow_message?: string;
  bootstrap_cmd: string | null;
};

function ConnectDialog({ onClose, onSaved }: { onClose: () => void; onSaved: () => void }) {
  const [repo, setRepo] = useState("");
  const [token, setToken] = useState("");
  const [note, setNote] = useState("");
  const [err, setErr] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [result, setResult] = useState<ConnectResult | null>(null);

  const submit = async () => {
    setErr(null); setResult(null); setSubmitting(true);
    try {
      const res = await api.connectRepo({
        repository: repo.trim(),
        token: token.trim(),
        note: note.trim() || undefined,
      });
      // Build the absolute one-liner the user pastes in their runner VM.
      // BACKEND_URL is "" when calls go through the dashboard's rewrite,
      // so default to window.location.origin for the bootstrap URL too.
      let bootstrapCmd: string | null = null;
      if (res.runner_bootstrap_url) {
        const origin = (typeof window !== "undefined" ? window.location.origin : "");
        bootstrapCmd = `curl -sSL ${origin}${res.runner_bootstrap_url} | bash`;
      }
      setResult({
        authenticated_as: res.authenticated_as,
        workflow_injected: res.workflow_injected,
        workflow_message: res.workflow_message,
        bootstrap_cmd: bootstrapCmd,
      });
    } catch (e) {
      setErr(String(e));
    } finally {
      setSubmitting(false);
    }
  };

  // Once the user dismisses the result panel we refresh the list.
  const done = () => { onSaved(); };

  return (
    <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-20" onClick={onClose}>
      <div className="bg-surface-card border border-surface-line rounded-md shadow-lg p-6 w-full max-w-xl" onClick={(e) => e.stopPropagation()}>
        {result ? (
          <ResultPanel result={result} onDone={done} />
        ) : (
          <>
            <h2 className="text-lg font-semibold mb-1">Connect a GitHub repository</h2>
            <p className="text-sm text-ink-muted mb-4">
              Paste a Personal Access Token only if you want Citadel to poll GitHub and auto-commit the workflow.
              Leave it blank when you are using a manual public-key or runner setup.
            </p>

            <div className="space-y-3 text-sm">
              <Field
                label="Repository"
                value={repo}
                setValue={setRepo}
                placeholder="owner/repo  (e.g. Mahesh-Kete/citadel)"
              />
              <Field
                label="Personal Access Token (optional)"
                value={token}
                setValue={setToken}
                placeholder="ghp_… or leave blank for manual/public-key setup"
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
                  Create a token for automatic GitHub polling →
                </a>
              </p>
              {err && (
                <div className="rounded border border-block-500/40 bg-block-50 p-2 mono text-block-700 text-xs">
                  {err}
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
                disabled={submitting || !repo}
                className="rounded bg-brand-600 px-3 py-1.5 text-white hover:bg-brand-700 disabled:opacity-50"
              >
                {submitting ? "Connecting…" : "Connect"}
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}

// copyText tries the Clipboard API first (only works on HTTPS/localhost), then
// falls back to the legacy execCommand path so plain-HTTP dashboards still work.
async function copyText(text: string): Promise<boolean> {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text);
      return true;
    }
  } catch { /* fall through */ }
  try {
    const ta = document.createElement("textarea");
    ta.value = text;
    ta.setAttribute("readonly", "");
    ta.style.position = "fixed";
    ta.style.top = "0";
    ta.style.left = "0";
    ta.style.opacity = "0";
    document.body.appendChild(ta);
    ta.focus();
    ta.select();
    ta.setSelectionRange(0, text.length);
    const ok = document.execCommand("copy");
    document.body.removeChild(ta);
    return ok;
  } catch {
    return false;
  }
}

function ResultPanel({ result, onDone }: { result: ConnectResult; onDone: () => void }) {
  const [copied, setCopied] = useState(false);
  const [copyErr, setCopyErr] = useState(false);
  const copy = async () => {
    if (!result.bootstrap_cmd) return;
    const ok = await copyText(result.bootstrap_cmd);
    if (ok) {
      setCopied(true);
      setCopyErr(false);
      setTimeout(() => setCopied(false), 1500);
    } else {
      setCopyErr(true);
      setTimeout(() => setCopyErr(false), 2500);
    }
  };

  return (
    <div>
      <h2 className="text-lg font-semibold mb-1 flex items-center gap-2">
        <CheckCircle2 className="h-5 w-5 text-ok-600" />
        Connected as <span className="mono">{result.authenticated_as}</span>
      </h2>
      <p className="text-sm text-ink-muted mb-4">
        {result.workflow_injected
          ? `Workflow committed: ${result.workflow_message ?? ".github/workflows/citadel.yml"}.`
          : `Workflow status: ${result.workflow_message ?? "skipped (already exists or non-fatal error)"}.`}
      </p>

      {result.bootstrap_cmd && (
        <div className="rounded border border-brand-500/40 bg-brand-50/40 p-3 mb-4">
          <div className="text-xs uppercase tracking-wide text-brand-700 mb-1.5 font-medium">
            One-time runner setup — paste this in your runner VM
          </div>
          <p className="text-xs text-ink-muted mb-2">
            Run this once on the machine that should execute CI jobs (skip if
            you already set up a runner for another repo on the same VM —
            future repos pick up the existing one automatically).
          </p>
          <div className="flex items-stretch gap-2">
            <pre className="flex-1 rounded border border-surface-line bg-surface-card px-3 py-2 text-xs mono overflow-x-auto whitespace-nowrap">
              {result.bootstrap_cmd}
            </pre>
            <button
              onClick={copy}
              className="rounded border border-surface-line bg-surface-card px-2.5 text-xs hover:bg-brand-50 whitespace-nowrap"
              title={copyErr ? "Copy failed — select the command manually" : "Copy to clipboard"}
            >
              {copyErr ? "Failed" : copied ? "Copied" : "Copy"}
            </button>
          </div>
        </div>
      )}

      <div className="flex justify-end">
        <button
          onClick={onDone}
          className="rounded bg-brand-600 px-3 py-1.5 text-white hover:bg-brand-700"
        >
          Done
        </button>
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
