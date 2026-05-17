# /examples

Sample workflows used for the demo.

## Files

- `victim-ci.yml` — a normal-looking Node.js build that runs on a `citadel-runner`-labeled self-hosted runner. Used as the baseline / "what a clean run looks like" demo.
- `attack-1-exfil.yml` — credential exfiltration. A malicious composite action `curl`s `$AWS_SECRET_ACCESS_KEY` to an attacker host. **Goal: Prevent Exfiltration.**
- `attack-2-revshell.yml` — reverse shell via a compromised npm `postinstall`. **Goal: Anomaly Detection.**
- `attack-3-tamper.yml` — source file modified between checkout and build. **Goal: Detect Tampering.**

Plus:

- `package.json`, `index.js`, `test.js` — minimal Node project so the victim workflow actually runs.
- `attacks/` — composite actions that implement each attack.

## Running

These workflows expect a self-hosted runner registered with the `citadel-runner` label. From the repo root:

```sh
gh workflow run victim-ci.yml
gh workflow run attack-1-exfil.yml
```

See `/docs/DEMO.md` for the full demo script.
