#!/usr/bin/env bash
#
# scripts/dev.sh — Run backend + detector + dashboard locally (no Docker).
#
# All three services share this terminal: their stdout is prefixed and
# interleaved. Ctrl-C cleanly stops every child by signalling the whole
# process group.
#
# Prereqs (one-time, run `make dev-setup`):
#   - Go 1.25+         (brew install go)
#   - Python 3.11+
#   - Node 20+         (brew install node)
#   - detector/.venv   created with pip-installed deps
#   - dashboard/node_modules installed
#   - backend/data     directory exists
#
# Ports:
#   8080  — backend HTTP API
#   8000  — detector HTTP API
#   3000  — dashboard (Next.js dev server)

set -euo pipefail

cd "$(dirname "$0")/.."

# ── colour helpers ────────────────────────────────────────────────────────
RED=$'\e[31m'; GRN=$'\e[32m'; BLU=$'\e[34m'; YEL=$'\e[33m'; RST=$'\e[0m'

abort() { echo "${RED}error:${RST} $*" >&2; exit 1; }

port_in_use() {
  # macOS-compatible port check
  lsof -i ":$1" -sTCP:LISTEN -P -n -t >/dev/null 2>&1
}

# ── preflight ─────────────────────────────────────────────────────────────
command -v go      >/dev/null || abort "Go not installed. Run: brew install go"
command -v node    >/dev/null || abort "Node not installed. Run: brew install node"
command -v python3 >/dev/null || abort "Python 3 not installed."

[ -d backend/data        ] || abort "backend/data missing. Run: make dev-setup"
[ -d detector/.venv      ] || abort "detector/.venv missing. Run: make dev-setup"
[ -d dashboard/node_modules ] || abort "dashboard/node_modules missing. Run: make dev-setup"

for port in 8080 8000 3000; do
  if port_in_use "$port"; then
    abort "port $port already in use. If Docker is running, stop it first: make docker-down"
  fi
done

# ── runtime ────────────────────────────────────────────────────────────────
# All children share this process group; `kill 0` on exit takes them down.
trap 'echo; echo "shutting down…"; kill 0 2>/dev/null || true; wait 2>/dev/null; exit 0' EXIT INT TERM

prefix() {
  # $1 = colored tag prefix
  while IFS= read -r line; do
    printf '%s %s\n' "$1" "$line"
  done
}

echo "${GRN}==> starting local Citadel stack${RST}"
echo "    backend   → http://localhost:8080"
echo "    detector  → http://localhost:8000"
echo "    dashboard → http://localhost:3000"
echo "    Ctrl-C    → stop everything"
echo

# Backend
(
  cd backend
  go run ./cmd/backend --addr=:8080 --db-path=./data/citadel.db 2>&1
) | prefix "${BLU}[backend] ${RST}" &

# Detector
(
  cd detector
  # shellcheck disable=SC1091
  source .venv/bin/activate
  exec uvicorn app.main:app --host 127.0.0.1 --port 8000 2>&1
) | prefix "${YEL}[detector]${RST}" &

# Dashboard — Next.js dev server proxies /api → backend via next.config.mjs
(
  cd dashboard
  export BACKEND_URL="http://localhost:8080"
  exec npm run dev 2>&1
) | prefix "${GRN}[dashboard]${RST}" &

wait
