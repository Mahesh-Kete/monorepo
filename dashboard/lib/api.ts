// Typed fetch wrappers around the Citadel backend.
//
// All calls happen client-side from the browser, so they use NEXT_PUBLIC_BACKEND_URL.

import type { ConnectedRepo, DetectionRow, Policy, RunDetail, RunSummary } from "./types";

export const BACKEND_URL = process.env.NEXT_PUBLIC_BACKEND_URL ?? "";

async function fetchJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const r = await fetch(`${BACKEND_URL}${path}`, {
    cache: "no-store",
    ...init,
    headers: { "Content-Type": "application/json", ...(init?.headers || {}) },
  });
  if (!r.ok) {
    throw new Error(`${r.status} ${r.statusText}: ${await r.text()}`);
  }
  return r.json() as Promise<T>;
}

export const api = {
  health: () => fetchJSON<{ status: string }>("/healthz"),
  listRuns: (limit = 50) => fetchJSON<RunSummary[]>(`/api/runs?limit=${limit}`),
  getRun: (id: number, type?: string) =>
    fetchJSON<RunDetail>(`/api/runs/${id}${type ? `?type=${type}` : ""}`),
  deleteRun: (id: number) =>
    fetchJSON<{ deleted: number }>(`/api/runs/${id}`, { method: "DELETE" }),
  deleteUnknownRuns: () =>
    fetchJSON<{ deleted: number }>(`/api/runs/unknown`, { method: "DELETE" }),
  getProcessTree: (id: number) =>
    fetchJSON<ProcessTreeNode[]>(`/api/runs/${id}/process-tree`),
  getBaselineDomains: (id: number) =>
    fetchJSON<string[]>(`/api/runs/${id}/baseline-domains`),
  listDetections: (since?: string) =>
    fetchJSON<DetectionRow[]>(`/api/detections${since ? `?since=${encodeURIComponent(since)}` : ""}`),
  listPolicies: () => fetchJSON<Policy[]>("/api/policies"),
  getPolicy: (id: number) => fetchJSON<Policy>(`/api/policies/${id}`),
  deletePolicy: (id: number) =>
    fetchJSON<{ deleted: number }>(`/api/policies/${id}`, { method: "DELETE" }),
  createPolicy: (p: Omit<Policy, "id" | "updated_at">) =>
    fetchJSON<{ id: number }>("/api/policies", {
      method: "POST",
      body: JSON.stringify(p),
    }),
  applicablePolicy: (repo: string, workflow: string) =>
    fetchJSON<Policy>(`/api/policies/applicable?repo=${encodeURIComponent(repo)}&workflow=${encodeURIComponent(workflow)}`),
  // --- Phase 11: Connect repo ---
  listRepos: () => fetchJSON<ConnectedRepo[]>(`/api/repos`),
  connectRepo: (body: { repository: string; token?: string; note?: string }) =>
    fetchJSON<{
      repository: string;
      authenticated_as: string;
      workflow_injected: boolean;
      workflow_message?: string;
      workflow_url?: string;
      repo_id?: number;
      runner_bootstrap_url?: string;
    }>(`/api/repos/connect`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  deleteRepo: (id: number) =>
    fetchJSON<{ deleted: number }>(`/api/repos/${id}`, { method: "DELETE" }),
  refreshRepo: (id: number) =>
    fetchJSON<{ fetched: number }>(`/api/repos/${id}/refresh`, { method: "POST" }),
};

export interface ProcessTreeNode {
  pid: number;
  ppid: number;
  comm: string;
  filename?: string;
  args?: string[];
  children?: ProcessTreeNode[];
}
