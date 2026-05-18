.PHONY: build-agent build-backend build-detector build-dashboard \
        docker-build docker-up docker-down \
        local-images demo-reset

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
