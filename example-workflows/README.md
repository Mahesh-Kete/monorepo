# example-workflows

Sample GitHub Actions workflows used for end-to-end demos of guardrail.

## Contents (planned)

- **victim/** — a realistic-looking workflow (e.g. `npm ci && npm test`,
  Python build, etc.) representing what a normal project runs.
- **attack/** — variants of the victim workflow that include a malicious
  step: exfiltrating secrets, beaconing to an attacker host, downloading a
  second-stage payload, etc.

These exist so we can demo:

1. Run the victim workflow with guardrail — observe a clean, allowlisted
   destination list.
2. Run the attack workflow with guardrail — observe the malicious call being
   captured, attributed to the offending step, and blocked.

## Status

Placeholder. No workflow YAML yet.
