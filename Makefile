# Local development. Mirrors the "Local development" section of the README;
# both Go binaries read .env from the working directory.

.DEFAULT_GOAL := help

# Throwaway database for the SQL suites (they TRUNCATE). Matches docker-compose.
TEST_DATABASE_URL ?= postgres://osint:osint@localhost:5433/osint_test
IMAGE ?= ghcr.io/mjudeikis/baltic-osint-hub
TAG ?= dev

.PHONY: help
help: ## List available targets
	@grep -hE '^[a-zA-Z0-9_-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  \033[36m%-13s\033[0m %s\n", $$1, $$2}'

.PHONY: db-up
db-up: ## Start Postgres (docker compose, localhost:5433)
	docker compose up -d

.PHONY: db-down
db-down: ## Stop Postgres (keeps the data volume)
	docker compose down

.PHONY: collect
collect: ## Run one collector fetch+classify cycle
	go run ./cmd/collector

web/node_modules: web/package.json web/package-lock.json
	cd web && npm install
	@touch web/node_modules

.PHONY: web
web: web/node_modules ## Build the frontend into web/dist
	cd web && npm run build

.PHONY: server
server: ## Run the API server on :8080 (serves web/dist)
	go run ./cmd/server

.PHONY: dev
dev: db-up web ## Postgres + fresh frontend build + server on :8080
	go run ./cmd/server

.PHONY: dev-web
dev-web: web/node_modules ## Vite dev server with hot reload (proxies /api to :8080)
	cd web && npm run dev

.PHONY: build
build: web ## Build both Go binaries into bin/ and the frontend
	go build -o bin/ ./cmd/...

.PHONY: docker-build
docker-build: ## Build the image locally (IMAGE=... TAG=...)
	docker build -t $(IMAGE):$(TAG) .

.PHONY: fmt
fmt: ## gofmt (+ goimports when installed) over the tree
	gofmt -w -s ./cmd ./internal
	@command -v goimports >/dev/null && goimports -w -local github.com/mjudeikis/baltic-osint-hub ./cmd ./internal || true

.PHONY: lint
lint: web/node_modules ## golangci-lint + frontend eslint and tsc
	golangci-lint run ./...
	cd web && npm run lint && npm run typecheck

.PHONY: helm-lint
helm-lint: ## Lint and render the chart in its main configurations
	helm lint deploy/helm/baltic-osint-hub
	helm template osint deploy/helm/baltic-osint-hub --set route.enabled=true --set postgres.backup.enabled=true >/dev/null
	helm template osint deploy/helm/baltic-osint-hub --set postgres.enabled=false >/dev/null

.PHONY: test
test: test-web ## Go tests that need no credentials, plus the frontend tests
	go test ./cmd/... ./internal/...

.PHONY: test-web
test-web: web/node_modules ## Frontend unit tests (vitest)
	cd web && npm test

.PHONY: test-db
test-db: ## SQL tests against a throwaway osint_test DB (TRUNCATEs it)
	PGPASSWORD=osint createdb -h localhost -p 5433 -U osint osint_test </dev/null 2>/dev/null || true
	TEST_DATABASE_URL=$(TEST_DATABASE_URL) go test ./cmd/... ./internal/...

.PHONY: test-live
test-live: ## SQL + live upstream feed tests (network; needs the throwaway DB)
	PGPASSWORD=osint createdb -h localhost -p 5433 -U osint osint_test </dev/null 2>/dev/null || true
	LIVE_FEEDS=1 TEST_DATABASE_URL=$(TEST_DATABASE_URL) go test ./cmd/... ./internal/...
