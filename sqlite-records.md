# SQLite Records Export

- VM: `citadel-u22-research` (`23.101.140.227`)
- Database: `/data/citadel.db`
- Exported: `2026-05-20T11:42:33Z`

## `connected_repos` (1 row)

| id | repository | token | note | created_at | last_polled_at | last_error |
| --- | --- | --- | --- | --- | --- | --- |
| 2 | Kiran-sec/citadel-victim | NULL | NULL | 2026-05-20 11:41:49 | 2026-05-20 11:42:10 | NULL |

## `detections` (4 rows)

| id | run_id | event_id | rule_name | severity | message | created_at |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | 1 | NULL | dns_block | high | blocked DNS query: hostname="results-receiver.actions.githubusercontent.com" — NXDOMAIN (not on Citadel DNS allowlist before any A/AAAA reply). | 2026-05-20 11:31:39 |
| 2 | 1 | NULL | base64_exfil | high | process bash(pid=40930, ppid=40927) — base64-exfil pattern (enforcement may apply): running `base64` on a CI runner (likely part of an exfil pipeline) | 2026-05-20 11:31:40 |
| 3 | 25 | NULL | dns_block | high | blocked DNS query: hostname="results-receiver.actions.githubusercontent.com" — NXDOMAIN (not on Citadel DNS allowlist before any A/AAAA reply). | 2026-05-20 11:39:40 |
| 4 | 25 | NULL | base64_exfil | high | process bash(pid=41537, ppid=41534) — base64-exfil pattern (enforcement may apply): running `base64` on a CI runner (likely part of an exfil pipeline) | 2026-05-20 11:39:41 |

## `events` (42 rows)

| id | run_id | type | timestamp | workflow_run_id | process | destination | blocked | filename |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 1 | network | 2026-05-20 11:31:39.329 +0000 UTC | 26159731412 | citadel-policy | 23.101.140.227:80 | false |  |
| 2 | 1 | network | 2026-05-20 11:31:39.489 +0000 UTC | 26159731412 | citadel-policy | 23.101.140.227:80 | false |  |
| 3 | 1 | network | 2026-05-20 11:31:39.652 +0000 UTC | 26159731412 | citadel-policy | 23.101.140.227:80 | false |  |
| 4 | 1 | network | 2026-05-20 11:31:39.809 +0000 UTC | 26159731412 | citadel-policy | 23.101.140.227:80 | false |  |
| 5 | 1 | network | 2026-05-20 11:31:39.961 +0000 UTC | 26159731412 | citadel-policy | 23.101.140.227:80 | false |  |
| 6 | 1 | process | 2026-05-20 11:31:40.034 +0000 UTC | 26159731412 | node |  |  | /usr/bin/git |
| 7 | 1 | network | 2026-05-20 11:31:40.053 +0000 UTC | 26159731412 | git-remote-http | 140.82.116.3:443 | false |  |
| 8 | 1 | network | 2026-05-20 11:31:40.122 +0000 UTC | 26159731412 | citadel-policy | 23.101.140.227:80 | false |  |
| 9 | 1 | network | 2026-05-20 11:31:40.279 +0000 UTC | 26159731412 | citadel-policy | 23.101.140.227:80 | false |  |
| 10 | 1 | network | 2026-05-20 11:31:40.438 +0000 UTC | 26159731412 | citadel-policy | 23.101.140.227:80 | false |  |
| 11 | 1 | process | 2026-05-20 11:31:40.520 +0000 UTC | 26159731412 | .NET TP Worker |  |  | /usr/bin/bash |
| 12 | 1 | process | 2026-05-20 11:31:40.528 +0000 UTC | 26159731412 | bash |  |  | base64 |
| 13 | 1 | network | 2026-05-20 11:31:40.528 +0000 UTC | 26159731412 | citadel-policy | 23.101.140.227:80 | false |  |
| 14 | 1 | network | 2026-05-20 11:31:40.598 +0000 UTC | 26159731412 | citadel-policy | 23.101.140.227:80 | false |  |
| 15 | 1 | network | 2026-05-20 11:31:40.757 +0000 UTC | 26159731412 | citadel-policy | 23.101.140.227:80 | false |  |
| 16 | 1 | network | 2026-05-20 11:31:40.914 +0000 UTC | 26159731412 | citadel-policy | 23.101.140.227:80 | false |  |
| 17 | 1 | process | 2026-05-20 11:31:41.011 +0000 UTC | 26159731412 | bash |  |  | grep |
| 18 | 1 | process | 2026-05-20 11:31:41.020 +0000 UTC | 26159731412 | bash |  |  | grep |
| 19 | 1 | process | 2026-05-20 11:31:41.028 +0000 UTC | 26159731412 | bash |  |  | grep |
| 20 | 1 | process | 2026-05-20 11:31:41.038 +0000 UTC | 26159731412 | bash |  |  | grep |
| 21 | 1 | process | 2026-05-20 11:31:41.043 +0000 UTC | 26159731412 | bash |  |  | grep |
| 22 | 25 | network | 2026-05-20 11:39:39.906 +0000 UTC | 26160115241 | citadel-policy | 23.101.140.227:80 | false |  |
| 23 | 25 | network | 2026-05-20 11:39:40.061 +0000 UTC | 26160115241 | citadel-policy | 23.101.140.227:80 | false |  |
| 24 | 25 | network | 2026-05-20 11:39:40.218 +0000 UTC | 26160115241 | citadel-policy | 23.101.140.227:80 | false |  |
| 25 | 25 | network | 2026-05-20 11:39:40.374 +0000 UTC | 26160115241 | citadel-policy | 23.101.140.227:80 | false |  |
| 26 | 25 | network | 2026-05-20 11:39:40.528 +0000 UTC | 26160115241 | citadel-policy | 23.101.140.227:80 | false |  |
| 27 | 25 | process | 2026-05-20 11:39:40.530 +0000 UTC | 26160115241 | node |  |  | /usr/bin/git |
| 28 | 25 | network | 2026-05-20 11:39:40.553 +0000 UTC | 26160115241 | git-remote-http | 140.82.116.3:443 | false |  |
| 29 | 25 | network | 2026-05-20 11:39:40.688 +0000 UTC | 26160115241 | citadel-policy | 23.101.140.227:80 | false |  |
| 30 | 25 | network | 2026-05-20 11:39:40.845 +0000 UTC | 26160115241 | citadel-policy | 23.101.140.227:80 | false |  |
| 31 | 25 | network | 2026-05-20 11:39:41.002 +0000 UTC | 26160115241 | citadel-policy | 23.101.140.227:80 | false |  |
| 32 | 25 | process | 2026-05-20 11:39:41.030 +0000 UTC | 26160115241 | .NET TP Worker |  |  | /usr/bin/bash |
| 33 | 25 | process | 2026-05-20 11:39:41.038 +0000 UTC | 26160115241 | bash |  |  | base64 |
| 34 | 25 | network | 2026-05-20 11:39:41.039 +0000 UTC | 26160115241 | citadel-policy | 23.101.140.227:80 | false |  |
| 35 | 25 | network | 2026-05-20 11:39:41.159 +0000 UTC | 26160115241 | citadel-policy | 23.101.140.227:80 | false |  |
| 36 | 25 | network | 2026-05-20 11:39:41.313 +0000 UTC | 26160115241 | citadel-policy | 23.101.140.227:80 | false |  |
| 37 | 25 | network | 2026-05-20 11:39:41.468 +0000 UTC | 26160115241 | citadel-policy | 23.101.140.227:80 | false |  |
| 38 | 25 | process | 2026-05-20 11:39:41.504 +0000 UTC | 26160115241 | bash |  |  | grep |
| 39 | 25 | process | 2026-05-20 11:39:41.515 +0000 UTC | 26160115241 | bash |  |  | grep |
| 40 | 25 | process | 2026-05-20 11:39:41.531 +0000 UTC | 26160115241 | bash |  |  | grep |
| 41 | 25 | process | 2026-05-20 11:39:41.545 +0000 UTC | 26160115241 | bash |  |  | grep |
| 42 | 25 | process | 2026-05-20 11:39:41.549 +0000 UTC | 26160115241 | bash |  |  | grep |

## `github_action_logs` (0 rows)

_No records._

## `policies` (0 rows)

_No records._

## `runs` (22 rows)

| id | repository | workflow | run_id | run_number | sha | ref | actor | started_at | policy_mode | status | gh_status | gh_conclusion | gh_html_url | gh_duration_sec | gh_event_name | gh_head_branch | gh_synced_at | agent_seen |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | kiran-sec/citadel-victim | Build PR (intentionally vulnerable) | 26159731412 | 42 | eec9bca9434b1d28bd54e221ac30709ce6da869e | refs/heads/main | 18kiran08 | 2026-05-20 11:31:39 | block | in_progress | NULL | NULL | NULL | NULL | NULL | NULL | NULL | 1 |
| 25 | kiran-sec/citadel-victim | Build PR (intentionally vulnerable) | 26160115241 | 43 | d6f6fe61bcb29b6a12b24e1484fe9547dfdb2edb | refs/heads/main | 18kiran08 | 2026-05-20 11:39:40 | block | in_progress | NULL | NULL | NULL | NULL | NULL | NULL | NULL | 1 |
| 26 | Kiran-sec/citadel-victim | Build PR (intentionally vulnerable) | 26160115241 | 43 | f8ee095f1a4f640ae7153b329bd8a67dd5f720e7 | refs/heads/attacker-base64-only | 18kiran08 | 2026-05-20 11:39:34 +0000 UTC | audit | in_progress | completed | success | https://github.com/kiran-sec/citadel-victim/actions/runs/26160115241 | 21 | pull_request_target | attacker-base64-only | 2026-05-20 11:42:09 | 0 |
| 27 | Kiran-sec/citadel-victim | Build PR (intentionally vulnerable) | 26159731412 | 42 | 88b560f27f9034ce145cee4f39e85ef403b8a961 | refs/heads/attacker-base64-only | 18kiran08 | 2026-05-20 11:31:33 +0000 UTC | audit | in_progress | completed | success | https://github.com/kiran-sec/citadel-victim/actions/runs/26159731412 | 23 | pull_request_target | attacker-base64-only | 2026-05-20 11:42:09 | 0 |
| 28 | Kiran-sec/citadel-victim | Build PR (intentionally vulnerable) | 26159527981 | 41 | 957e171a37d59275b1febf825f76d7efa93eeaa3 | refs/heads/attacker-base64-only | 18kiran08 | 2026-05-20 11:27:23 +0000 UTC | audit | in_progress | completed | success | https://github.com/kiran-sec/citadel-victim/actions/runs/26159527981 | 22 | pull_request_target | attacker-base64-only | 2026-05-20 11:42:09 | 0 |
| 29 | Kiran-sec/citadel-victim | Build PR (intentionally vulnerable) | 26155662517 | 40 | bc5eb7ba19bad56bebf0039c789eb02e3a4338d3 | refs/heads/attacker-base64-only | 18kiran08 | 2026-05-20 10:05:52 +0000 UTC | audit | in_progress | completed | failure | https://github.com/kiran-sec/citadel-victim/actions/runs/26155662517 | 21 | pull_request_target | attacker-base64-only | 2026-05-20 11:42:09 | 0 |
| 30 | Kiran-sec/citadel-victim | Build PR (intentionally vulnerable) | 26154813705 | 39 | e91b41de8eae6f30dc8cf4c6b4864177b11e96f6 | refs/heads/attacker-base64-only | 18kiran08 | 2026-05-20 09:48:34 +0000 UTC | audit | in_progress | completed | failure | https://github.com/kiran-sec/citadel-victim/actions/runs/26154813705 | 22 | pull_request_target | attacker-base64-only | 2026-05-20 11:42:09 | 0 |
| 31 | Kiran-sec/citadel-victim | Build PR (intentionally vulnerable) | 26154318910 | 38 | b2bf1b84deafaf24e6d98df186064000a177712a | refs/heads/attacker-base64-only | 18kiran08 | 2026-05-20 09:38:34 +0000 UTC | audit | in_progress | completed | failure | https://github.com/kiran-sec/citadel-victim/actions/runs/26154318910 | 57 | pull_request_target | attacker-base64-only | 2026-05-20 11:42:09 | 0 |
| 32 | Kiran-sec/citadel-victim | Build PR (intentionally vulnerable) | 26153329713 | 37 | c038cc1b97a6ddd14ae5e76502959f877b005efd | refs/heads/attacker-base64-only | 18kiran08 | 2026-05-20 09:18:41 +0000 UTC | audit | in_progress | completed | failure | https://github.com/kiran-sec/citadel-victim/actions/runs/26153329713 | 23 | pull_request_target | attacker-base64-only | 2026-05-20 11:42:09 | 0 |
| 33 | Kiran-sec/citadel-victim | Build PR (intentionally vulnerable) | 26152792155 | 36 | 8b7fd79d9a8b8d9eb43b72685a4ae19f1dee38ea | refs/heads/attacker-base64-only | 18kiran08 | 2026-05-20 09:07:50 +0000 UTC | audit | in_progress | completed | failure | https://github.com/kiran-sec/citadel-victim/actions/runs/26152792155 | 23 | pull_request_target | attacker-base64-only | 2026-05-20 11:42:09 | 0 |
| 34 | Kiran-sec/citadel-victim | Build PR (intentionally vulnerable) | 26151862745 | 35 | 6c0b2ab0f9ed5b00c29789892a22b0a901a70eec | refs/heads/attacker-network-only | 18kiran08 | 2026-05-20 09:03:51 +0000 UTC | audit | in_progress | completed | failure | https://github.com/kiran-sec/citadel-victim/actions/runs/26151862745 | 24 | pull_request_target | attacker-network-only | 2026-05-20 11:42:09 | 0 |
| 35 | Kiran-sec/citadel-victim | Build PR (intentionally vulnerable) | 26150953997 | 34 | 25eb5e9a7b2cd5bf17f478495f6741545d23559b | refs/heads/attacker-base64-only | 18kiran08 | 2026-05-20 08:30:39 +0000 UTC | audit | in_progress | completed | failure | https://github.com/kiran-sec/citadel-victim/actions/runs/26150953997 | 22 | pull_request_target | attacker-base64-only | 2026-05-20 11:42:09 | 0 |
| 36 | Kiran-sec/citadel-victim | Build PR (intentionally vulnerable) | 26150865590 | 33 | 974ef3bc7dcd5a12a57220c4ad92d2053ab50c4c | refs/heads/attacker-network-only | 18kiran08 | 2026-05-20 08:28:53 +0000 UTC | audit | in_progress | completed | success | https://github.com/kiran-sec/citadel-victim/actions/runs/26150865590 | 23 | pull_request_target | attacker-network-only | 2026-05-20 11:42:09 | 0 |
| 37 | Kiran-sec/citadel-victim | Build PR (intentionally vulnerable) | 26150621550 | 32 | 032b4c5230cc690acee45c9d0f17f43b22490bd7 | refs/heads/block-mode-rollout | kiran-sec | 2026-05-20 08:23:44 +0000 UTC | audit | in_progress | completed | success | https://github.com/kiran-sec/citadel-victim/actions/runs/26150621550 | 22 | workflow_dispatch | block-mode-rollout | 2026-05-20 11:42:09 | 0 |
| 38 | Kiran-sec/citadel-victim | Build PR (intentionally vulnerable) | 26150414309 | 31 | 032b4c5230cc690acee45c9d0f17f43b22490bd7 | refs/heads/block-mode-rollout | kiran-sec | 2026-05-20 08:19:17 +0000 UTC | audit | in_progress | completed | failure | https://github.com/kiran-sec/citadel-victim/actions/runs/26150414309 | 36 | workflow_dispatch | block-mode-rollout | 2026-05-20 11:42:09 | 0 |
| 39 | Kiran-sec/citadel-victim | Build PR (intentionally vulnerable) | 26150193024 | 30 | 032b4c5230cc690acee45c9d0f17f43b22490bd7 | refs/heads/block-mode-rollout | kiran-sec | 2026-05-20 08:14:37 +0000 UTC | audit | in_progress | completed | failure | https://github.com/kiran-sec/citadel-victim/actions/runs/26150193024 | 12 | workflow_dispatch | block-mode-rollout | 2026-05-20 11:42:09 | 0 |
| 40 | Kiran-sec/citadel-victim | Build PR (intentionally vulnerable) | 26148465120 | 29 | 032b4c5230cc690acee45c9d0f17f43b22490bd7 | refs/heads/block-mode-rollout | kiran-sec | 2026-05-20 07:37:42 +0000 UTC | audit | in_progress | completed | failure | https://github.com/kiran-sec/citadel-victim/actions/runs/26148465120 | 10 | workflow_dispatch | block-mode-rollout | 2026-05-20 11:42:09 | 0 |
| 41 | Kiran-sec/citadel-victim | Build PR (intentionally vulnerable) | 26148309699 | 28 | 032b4c5230cc690acee45c9d0f17f43b22490bd7 | refs/heads/block-mode-rollout | kiran-sec | 2026-05-20 07:34:15 +0000 UTC | audit | in_progress | completed | failure | https://github.com/kiran-sec/citadel-victim/actions/runs/26148309699 | 12 | workflow_dispatch | block-mode-rollout | 2026-05-20 11:42:09 | 0 |
| 42 | Kiran-sec/citadel-victim | Build PR (intentionally vulnerable) | 26148126429 | 27 | 032b4c5230cc690acee45c9d0f17f43b22490bd7 | refs/heads/block-mode-rollout | kiran-sec | 2026-05-20 07:30:19 +0000 UTC | audit | in_progress | completed | failure | https://github.com/kiran-sec/citadel-victim/actions/runs/26148126429 | 11 | workflow_dispatch | block-mode-rollout | 2026-05-20 11:42:09 | 0 |
| 43 | Kiran-sec/citadel-victim | Build PR (intentionally vulnerable) | 26144880277 | 26 | d40a3404a1dfd30be80779c7cc54388e6aed316d | refs/heads/attacker-base64-exfil | 18kiran08 | 2026-05-20 06:11:39 +0000 UTC | audit | in_progress | completed | success | https://github.com/kiran-sec/citadel-victim/actions/runs/26144880277 | 17 | pull_request_target | attacker-base64-exfil | 2026-05-20 11:42:09 | 0 |
| 44 | Kiran-sec/citadel-victim | Build PR (intentionally vulnerable) | 26144605531 | 25 | 181645103de3e5ca568d863f8b0d43ee3b280264 | refs/heads/attacker-base64-exfil | 18kiran08 | 2026-05-20 06:04:05 +0000 UTC | audit | in_progress | completed | success | https://github.com/kiran-sec/citadel-victim/actions/runs/26144605531 | 17 | pull_request_target | attacker-base64-exfil | 2026-05-20 11:42:09 | 0 |
| 45 | Kiran-sec/citadel-victim | Build PR (intentionally vulnerable) | 26143947269 | 24 | 3f87d82fef4412d43234246599987070a0bcf160 | refs/heads/attacker-base64-exfil | 18kiran08 | 2026-05-20 05:45:53 +0000 UTC | audit | in_progress | completed | success | https://github.com/kiran-sec/citadel-victim/actions/runs/26143947269 | 965 | pull_request_target | attacker-base64-exfil | 2026-05-20 11:42:09 | 0 |
