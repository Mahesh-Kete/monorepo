.PHONY: build-agent build-backend build-detector build-dashboard \
        docker-build docker-up docker-down \
        local-images demo-reset \
        dev dev-setup dev-backend dev-detector dev-dashboard dev-clean

build-agent:
	$(MAKE) -C agent build

build-backend:
	cd backend && go build -o bin/citadel-backend ./cmd/backend

build-detector:
	cd detector && python -m pip install -e .

build-dashboard:
	cd dashboard && npm install && npm run build

docker-build:
	docker compose build

docker-up:
	docker compose up -d

docker-down:
	docker compose down

# Build all four service images locally and tag them as :dev so the composite
# action (action/action.yml) can be invoked with `image-tag: dev` and skip
# the docker pull. Useful for demo days without depending on GHCR.
local-images:
	docker build -t citadel-agent:dev     ./agent
	docker build -t citadel-backend:dev   ./backend
	docker build -t citadel-detector:dev  ./detector
	docker build -t citadel-dashboard:dev ./dashboard
	@echo
	@docker images --filter "reference=citadel-*:dev"
	@echo "All four images built. Use with action: 'image-tag: dev'."

# =============================================================================
# Local dev (no Docker) — runs backend + detector + dashboard natively on Mac
# =============================================================================

# One-time setup: install Python venv, npm deps, create backend data dir.
# Re-runnable safely.
dev-setup:
	@echo "==> backend: ensuring data dir"
	@mkdir -p backend/data
	@echo "==> detector: creating venv + installing deps"
	@if [ ! -d detector/.venv ]; then \
		cd detector && python3 -m venv .venv; \
	fi
	@cd detector && . .venv/bin/activate && pip install --quiet --upgrade pip \
		&& pip install --quiet fastapi 'uvicorn[standard]>=0.27' httpx 'pydantic>=2.6' \
		   python-dateutil pyyaml
	@echo "==> dashboard: npm install"
	@cd dashboard && npm install --no-audit --no-fund --silent
	@echo
	@echo "✓ dev-setup complete. Run 'make dev' to start everything."

# Run all three services in parallel, sharing this terminal. Ctrl-C stops all.
dev:
	@./scripts/dev.sh

# Run an individual service standalone (useful when iterating on one piece).
dev-backend:
	@cd backend && go run ./cmd/backend --addr=:8080 --db-path=./data/citadel.db

dev-detector:
	@cd detector && . .venv/bin/activate && uvicorn app.main:app --host 127.0.0.1 --port 8000

dev-dashboard:
	@cd dashboard && npm run dev

# Nuke local-dev state: SQLite DB, Python venv, node_modules.
dev-clean:
	@rm -rf backend/data detector/.venv dashboard/node_modules dashboard/.next
	@echo "✓ local dev state wiped"

# =============================================================================
# Demo reset
# =============================================================================

# Wipe state from previous demo runs so the next demo starts clean.
# Linux-only bits (iptables) are guarded; failures are non-fatal.
demo-reset:
	@echo "==> demo-reset: cleaning state"
	-docker stop citadel 2>/dev/null
	-docker rm   citadel 2>/dev/null
	-rm -f backend/citadel.db backend/citadel.db-wal backend/citadel.db-shm
	-rm -f detector/data/baseline.json detector/data/state.json
	-rm -f /tmp/citadel-* /tmp/before*.json /tmp/baseline*.json
	@if command -v iptables >/dev/null 2>&1; then \
		sudo iptables -t nat -F OUTPUT 2>/dev/null || true; \
		sudo iptables -F OUTPUT       2>/dev/null || true; \
	fi
	@echo "==> Ready. Demo your magic."
