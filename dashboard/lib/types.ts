// Types mirroring the Go Event schema and backend response shapes.
// Keep in sync with /backend/internal/api/*.go.

export type Severity = "info" | "low" | "medium" | "high" | "critical";
export type Allowed = "allowed" | "blocked";

export interface NetworkData {
  src_ip?: string;
  dst_ip: string;
  dst_port: number;
  hostname?: string;
  process?: string;
  // Optional: agent in block mode sets `blocked: true` when the egress
  // packet was dropped at cgroup_skb. Absent on audit-mode events.
  blocked?: boolean;
}

export interface ProcessData {
  pid: number;
  ppid: number;
  uid: number;
  comm: string;
  filename?: string;
  args?: string[];
}

export interface FileData {
  path: string;
  flags?: string;
  old_hash?: string;
  new_hash?: string;
  action?: string;
}

export interface WorkflowMeta {
  repository?: string;
  workflow?: string;
  workflow_file?: string;
  run_id?: string;
  run_number?: string;
  sha?: string;
  ref?: string;
  actor?: string;
  event_name?: string;
  job?: string;
  step?: string;
}

export interface CitadelEvent {
  id: string;
  type: string;
  timestamp: string;
  network?: NetworkData;
  process?: ProcessData;
  file?: FileData;
  process_chain?: string[];
  workflow: WorkflowMeta;
}

export interface RunSummary {
  id: number;
  repository: string;
  workflow?: string;
  run_id: string;
  run_number?: string;
  sha?: string;
  ref?: string;
  actor?: string;
  started_at: string;
  policy_mode: string;
  status: string;
  event_counts: { network: number; process: number; file: number; file_tamper: number };
  detection_count: number;
  severity_max?: string;
  // Populated by the GitHub Actions poller (Phase 11 "Connect repo")
  gh_status?: string;       // queued | in_progress | completed
  gh_conclusion?: string;   // success | failure | cancelled | …
  gh_html_url?: string;
  gh_duration_sec?: number;
  gh_event_name?: string;
  gh_head_branch?: string;
  gh_synced_at?: string;
  agent_seen: boolean;      // true iff Citadel agent ingested any events
}

export interface ConnectedRepo {
  id: number;
  repository: string;
  note?: string;
  created_at: string;
  last_polled_at?: string;
  last_error?: string;
}

export interface DetectionRow {
  id: number;
  run_id: number;
  event_id?: number;
  rule_name: string;
  severity: Severity;
  message?: string;
  created_at: string;
}

export interface RunDetail {
  run: RunSummary;
  events: CitadelEvent[];
  detections: DetectionRow[];
}

export interface Policy {
  id: number;
  name: string;
  scope_repo?: string;
  scope_workflow?: string;
  mode: "audit" | "block";
  allowlist?: string[];
  detection_rules?: Record<string, string>;
  updated_at?: string;
}
