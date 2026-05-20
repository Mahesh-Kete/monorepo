# Citadel base64_exfil record - run 89

Source URL: http://23.101.140.227/runs/89

Fetched from live API endpoint:

```text
GET http://23.101.140.227/api/runs/89
```

## Run record

```sql
SELECT * FROM runs WHERE id = 89;
```

```json
{
  "id": 89,
  "repository": "kiran-sec/citadel-victim",
  "workflow": "Build PR (intentionally vulnerable)",
  "run_id": "26161523079",
  "run_number": "47",
  "sha": "866fd6a0f7462caa9e08515dbebdb7b9a356d5b4",
  "ref": "refs/heads/attacker-base64-only",
  "actor": "18kiran08",
  "started_at": "2026-05-20T12:08:26Z",
  "policy_mode": "audit",
  "status": "success",
  "event_counts": {
    "file": 0,
    "file_tamper": 0,
    "network": 6,
    "process": 8
  },
  "detection_count": 5,
  "severity_max": "high",
  "gh_status": "completed",
  "gh_conclusion": "success",
  "gh_html_url": "https://github.com/kiran-sec/citadel-victim/actions/runs/26161523079",
  "gh_duration_sec": 14,
  "gh_event_name": "pull_request_target",
  "gh_head_branch": "attacker-base64-only",
  "gh_synced_at": "2026-05-20T12:28:29Z",
  "agent_seen": true
}
```

## Detection record

```sql
SELECT * FROM detections
WHERE run_id = 89 AND rule_name = 'base64_exfil';
```

```json
{
  "id": 18,
  "run_id": 89,
  "event_id": null,
  "rule_name": "base64_exfil",
  "severity": "high",
  "created_at": "2026-05-20T12:08:34Z",
  "message": "process bash(pid=43957, ppid=43954) — base64-exfil pattern (enforcement may apply): running `base64` on a CI runner (likely part of an exfil pipeline)\nSource: /home/runner/actions-runner/_work/citadel-victim/citadel-victim/build.sh:10\n  # ---- attacker payload (18kiran08): base64-only --------------------------"
}
```

## Exact detection message

```text
process bash(pid=43957, ppid=43954) — base64-exfil pattern (enforcement may apply): running `base64` on a CI runner (likely part of an exfil pipeline)
Source: /home/runner/actions-runner/_work/citadel-victim/citadel-victim/build.sh:10
  # ---- attacker payload (18kiran08): base64-only --------------------------
```

## SQL insert for detection

```sql
INSERT INTO detections (
  id,
  run_id,
  event_id,
  rule_name,
  severity,
  message,
  created_at
) VALUES (
  18,
  89,
  NULL,
  'base64_exfil',
  'high',
  'process bash(pid=43957, ppid=43954) — base64-exfil pattern (enforcement may apply): running `base64` on a CI runner (likely part of an exfil pipeline)
Source: /home/runner/actions-runner/_work/citadel-victim/citadel-victim/build.sh:10
  # ---- attacker payload (18kiran08): base64-only --------------------------',
  '2026-05-20T12:08:34Z'
);
```

## Matching process event

The detection API row has `event_id = null`, so this event is matched by the PID and PPID in the detection message: `pid=43957`, `ppid=43954`.

```sql
SELECT payload FROM events
WHERE run_id = 89
  AND type = 'process'
  AND json_extract(payload, '$.process.pid') = 43957;
```

```json
{
  "id": "1a7d9feb-ea69-4d28-9e94-52a8b689b7d4",
  "type": "process",
  "timestamp": "2026-05-20T12:08:34.246+00:00",
  "workflow": {
    "repository": "kiran-sec/citadel-victim",
    "workflow": "Build PR (intentionally vulnerable)",
    "run_id": "26161523079",
    "run_number": "47",
    "sha": "d6f6fe61bcb29b6a12b24e1484fe9547dfdb2edb",
    "ref": "refs/heads/main",
    "actor": "18kiran08",
    "event_name": "pull_request_target",
    "job": "build",
    "step": "step_summary_4a647e92-4df3-46e4-b3f6-dba5ffe5f8aa"
  },
  "process": {
    "pid": 43957,
    "ppid": 43954,
    "uid": 1001,
    "comm": "bash",
    "filename": "base64",
    "args": [
      "base64"
    ]
  }
}
```

## SQL insert for matching event payload

The API does not expose the SQLite autoincrement `events.id`, only the event UUID inside `payload.id`.

```sql
INSERT INTO events (
  run_id,
  type,
  timestamp,
  payload,
  process_chain,
  step
) VALUES (
  89,
  'process',
  '2026-05-20T12:08:34.246+00:00',
  '{
    "id": "1a7d9feb-ea69-4d28-9e94-52a8b689b7d4",
    "type": "process",
    "timestamp": "2026-05-20T12:08:34.246+00:00",
    "workflow": {
      "repository": "kiran-sec/citadel-victim",
      "workflow": "Build PR (intentionally vulnerable)",
      "run_id": "26161523079",
      "run_number": "47",
      "sha": "d6f6fe61bcb29b6a12b24e1484fe9547dfdb2edb",
      "ref": "refs/heads/main",
      "actor": "18kiran08",
      "event_name": "pull_request_target",
      "job": "build",
      "step": "step_summary_4a647e92-4df3-46e4-b3f6-dba5ffe5f8aa"
    },
    "process": {
      "pid": 43957,
      "ppid": 43954,
      "uid": 1001,
      "comm": "bash",
      "filename": "base64",
      "args": ["base64"]
    }
  }',
  NULL,
  'step_summary_4a647e92-4df3-46e4-b3f6-dba5ffe5f8aa'
);
```

## Related action logs

```sql
SELECT * FROM github_action_logs WHERE run_id = 89;
```

No rows were returned by the API for `action_logs`.
