# action

Composite GitHub Action wrapper that drops guardrail into any workflow.

## Responsibilities

- Install the `/agent` binary on the runner at job start.
- Start the agent and point it at the configured backend.
- Emit step-boundary markers so the agent can attribute events to the right
  workflow step.
- Tear down cleanly at job end and flush remaining events.

## Usage (planned)

```yaml
- uses: ./action
  with:
    backend-url: https://guardrail.example.com
    policy: strict
```

## Status

Placeholder. No `action.yml` yet.
