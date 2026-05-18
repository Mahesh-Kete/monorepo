# Linux Self-Hosted Runner Setup

End-to-end instructions for turning a Linux laptop / VM into a GitHub
Actions self-hosted runner that can host the full Citadel demo. Follow this
top-to-bottom on **the Linux box** (not your Mac).

## Prerequisites

- Ubuntu 22.04+ (or any distro with kernel ≥ 5.15 and BTF enabled)
- A user with passwordless `sudo` (the runner agent needs it)
- ~10 GB free disk (Docker images + Go + Node + Python toolchains)
- Network access to github.com and ghcr.io

Verify the kernel and BTF:

```sh
uname -r                       # must be ≥ 5.15
ls -la /sys/kernel/btf/vmlinux # must exist
```

If either fails, eBPF won't work and the whole demo is moot. On Ubuntu 22.04
LTS both are guaranteed.

## Step 1 — Install everything the build needs

```sh
sudo apt-get update -qq
sudo apt-get install -y --no-install-recommends \
    build-essential clang llvm libelf-dev libbpf-dev \
    linux-headers-$(uname -r) linux-tools-generic linux-tools-common \
    make pkg-config curl git ca-certificates jq

# Symlink bpftool to a stable path
sudo ln -sf $(ls /usr/lib/linux-tools/*/bpftool 2>/dev/null | head -1) /usr/local/bin/bpftool
bpftool version

# Docker (skip if already present)
if ! command -v docker >/dev/null; then
  curl -fsSL https://get.docker.com | sudo sh
  sudo usermod -aG docker $USER
  echo "Log out + back in so your docker group membership takes effect."
fi

# Go 1.25 (replaces any system go)
GOARCH=$(uname -m | sed 's/x86_64/amd64/; s/aarch64/arm64/')
cd /tmp
curl -fsSLO https://go.dev/dl/go1.25.0.linux-${GOARCH}.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.25.0.linux-${GOARCH}.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
export PATH=$PATH:/usr/local/go/bin
go version

# Node 20 (for the dashboard image build only; not needed at runtime)
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt-get install -y nodejs
node --version
```

## Step 2 — Clone the repo

```sh
cd ~
git clone https://github.com/Mahesh-Kete/citadel.git
cd citadel
```

## Step 3 — Generate `vmlinux.h` for *this* kernel

This file is BTF — it pins the eBPF programs' field offsets to the running
kernel. Different kernels produce different files, so it has to be generated
on the exact box where the agent will run.

```sh
sudo bpftool btf dump file /sys/kernel/btf/vmlinux format c > agent/bpf/vmlinux.h
wc -l agent/bpf/vmlinux.h   # tens of thousands of lines
```

Commit it so future runs don't have to regenerate (kernel-specific, but
useful to have in the repo for reproducibility):

```sh
git add agent/bpf/vmlinux.h
git -c user.name="$(git config user.name)" -c user.email="$(git config user.email)" \
    commit -m "Add vmlinux.h for kernel $(uname -r)"
git push citadel main   # or `origin main` if you set this repo's origin
```

## Step 4 — Build all 4 service images locally

```sh
make local-images
```

You should end up with:

```text
citadel-agent:dev       ~150 MB
citadel-backend:dev      ~25 MB
citadel-detector:dev    ~200 MB
citadel-dashboard:dev   ~250 MB
```

The first build is ~3-5 minutes (downloading bases + compiling). Subsequent
builds use Docker's layer cache and are seconds.

## Step 5 — Register this box as a self-hosted runner

Go to **https://github.com/Mahesh-Kete/citadel/settings/actions/runners/new**
in a browser. Pick **Linux** and your CPU arch. GitHub gives you a series of
commands that look like:

```sh
# (Paste the EXACT commands GitHub gives — they include a one-time token.)
mkdir actions-runner && cd actions-runner
curl -o actions-runner-linux-x64-…tar.gz -L https://github.com/actions/runner/releases/download/v…
./config.sh --url https://github.com/Mahesh-Kete/citadel --token <YOUR-ONE-TIME-TOKEN>
```

When `config.sh` asks for **runner labels**, type:

```text
citadel-runner
```

(That's the label the workflow files in `examples/` are looking for.)

Then start the runner:

```sh
./run.sh
```

Or install it as a service (recommended for the demo so it survives reboots):

```sh
sudo ./svc.sh install
sudo ./svc.sh start
```

Verify it appears as Online in the GitHub Settings → Actions → Runners
page.

## Step 6 — Bring up the rest of the stack

```sh
cd ~/citadel
make docker-up
```

This starts the backend (`:8080`), detector (`:8000`), and dashboard
(`:3000`). The agent does NOT run here — it runs *inside* every workflow
via the composite action.

Verify:

```sh
curl http://localhost:8080/healthz   # backend
curl http://localhost:8000/healthz   # detector
xdg-open http://localhost:3000       # dashboard (or visit from your laptop)
```

## Step 7 — Trigger an attack demo

From your Mac (or any machine with `gh`):

```sh
gh workflow run attack-1-exfil.yml --repo Mahesh-Kete/citadel
gh workflow run attack-2-revshell.yml --repo Mahesh-Kete/citadel
gh workflow run attack-3-tamper.yml --repo Mahesh-Kete/citadel
```

Or click "Run workflow" on each in the GitHub Actions UI.

The workflows run on your self-hosted runner. Watch the dashboard at
`http://<runner-ip>:3000/runs` — within ~5 seconds of the workflow firing
the first event, you'll see a new run appear with detections in the
sidebar.

## Step 8 — Reset between demo attempts

```sh
make demo-reset
make docker-up
```

That nukes the SQLite DB, baseline state, iptables rules, agent container.
Run before each rehearsal.

---

## Troubleshooting

| Symptom | Likely cause | Fix |
| :--- | :--- | :--- |
| `vmlinux.h not found` during `make local-images` | Step 3 wasn't run | Run step 3 and retry |
| Workflow stuck "Waiting for a runner to pick up this job" | Runner not running or wrong label | `cd actions-runner && ./run.sh` — confirm label is `citadel-runner` |
| BPF verifier rejection on agent startup | `vmlinux.h` was generated on a different kernel | Regenerate on this box |
| Agent container exits immediately | Missing `--privileged` or a wrong cgroup path | `docker logs citadel` — usually obvious |
| Dashboard shows red status dot | Backend container down | `docker compose ps` and `docker compose logs backend` |
| `attacker.example.com` doesn't resolve | That's intentional — the attack doesn't need to actually exfil; the *attempt* fires the probes | Not a bug |
| `npm install` fails inside attack-2 because of the postinstall reverse-shell | Expected and benign — npm reports postinstall exit code but the workflow continues | Leave it |

See `docs/DEMO.md` for the live demo script and `docs/local-demo.md` for
the no-runner local-only flow.
