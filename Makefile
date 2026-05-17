.PHONY: build-agent build-backend build-detector build-dashboard \
        docker-build docker-up docker-down demo-reset

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

demo-reset:
	@echo "demo-reset: stub — implemented in Phase 10"
