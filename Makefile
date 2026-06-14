.PHONY: build run test test-unit test-integration test-e2e docker-up docker-down clean

build:
	go build -o bin/shortlink ./cmd/server

run:
	go run ./cmd/server

# --- Testing -----------------------------------------------------------
# Unit tests: no external dependencies, always runnable.
test-unit:
	go test ./internal/... -v -count=1 -race

# Integration tests: require PostgreSQL and Redis.
# Start deps first:  make docker-deps
# Then run:          make test-integration
test-integration:
	go test ./tests/ -v -count=1 -race

# Run both unit + integration (requires deps running).
test: test-unit test-integration

# Docker-based full E2E smoke test (builds + starts everything in Docker).
test-e2e:
	bash tests/e2e_docker_test.sh

# Start only PG + Redis for local development / integration tests.
docker-deps:
	docker compose up postgres redis -d
	@echo "Waiting for services..."
	@sleep 3
	@echo "Ready. Run: make test-integration"

# --- Docker full stack -------------------------------------------------
docker-up:
	docker compose up --build -d

docker-down:
	docker compose down

clean:
	rm -rf bin/
