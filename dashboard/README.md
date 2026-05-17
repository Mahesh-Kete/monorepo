# dashboard

Next.js UI for guardrail.

## Responsibilities

- Show recent GitHub Actions runs and the network activity observed in each.
- Drill into a run: per-step DNS queries, connection attempts, and policy
  decisions (allowed / logged / blocked).
- Surface incidents — workflows that tried to reach destinations outside the
  policy.
- Provide a policy editor (allowlist of destinations / domains) and push
  updates to the backend.

## Tech

- Next.js (App Router).
- Talks to `/backend` over HTTP.

## Status

Placeholder. No code yet.
