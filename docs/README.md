# docs

Architecture notes, design decisions, and threat model for guardrail.

## Planned contents

- `architecture.md` — system overview: agent ↔ backend ↔ dashboard, and how
  the composite action ties them into a workflow run.
- `threat-model.md` — what guardrail does and doesn't defend against; trust
  boundaries; assumptions about the runner.
- `policy.md` — policy document format (allowlist schema, match semantics,
  resolution order).
- `step-correlation.md` — how network events get attributed to a specific
  workflow step.
- `decisions/` — short ADR-style notes for non-obvious choices.

## Status

Placeholder. No docs written yet.
