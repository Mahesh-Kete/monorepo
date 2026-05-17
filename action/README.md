# /action

`citadel-setup` — composite GitHub Action that drops Citadel into any workflow as a single step.

## What it does

1. Writes `GITHUB_*` metadata to `/tmp/citadel-meta.json`.
2. Installs the `citadel-step` shell helper so later steps can mark which step they belong to (writes to `/tmp/citadel-current-step`).
3. Snapshots the workspace immediately after `actions/checkout` (SHA256 per file) for tampering detection.
4. Starts the `citadel-agent` container with `--privileged --pid=host --network=host`.
5. On `post:` (always), diffs the workspace against the baseline, reports findings, and tears the agent down.

## Inputs

| Name         | Default                | Description                                |
| :----------- | :--------------------- | :----------------------------------------- |
| `mode`       | `audit`                | `audit` or `block`                         |
| `backend-url`| (required)             | Where the agent ships events               |
| `image-tag`  | `latest`               | Citadel agent image tag                    |
| `watch-path` | `$GITHUB_WORKSPACE`    | Root for workspace tampering detection     |

## Usage

```yaml
- uses: <org>/citadel/action@v1
  with:
    backend-url: ${{ secrets.CITADEL_BACKEND_URL }}
    mode: audit
```
