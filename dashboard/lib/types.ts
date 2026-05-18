// Types mirroring the Go Event schema and backend response shapes.
// Keep in sync with /backend/internal/api/*.go.

export type Severity = "info" | "low" | "medium" | "high" | "critical";

export interface NetworkData {
  src_ip?: string;
  dst_ip: string;
  dst_port: number;
  hostname?: string;
  process?: string;
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
