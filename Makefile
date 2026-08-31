# Local development. Mirrors the "Local development" section of the README;
# both Go binaries read .env from the working directory.

.DEFAULT_GOAL := help

.PHONY: help
help: ## List available targets
	@grep -hE '^[a-zA-Z0-9_-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

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

.PHONY: test
test: ## Run the Go tests that need no credentials
	go test ./...

.PHONY: test-db
test-db: ## SQL tests against a throwaway osint_test DB (TRUNCATEs it)
	createdb -h localhost -p 5433 -U osint osint_test 2>/dev/null || true
	TEST_DATABASE_URL=postgres://osint:osint@localhost:5433/osint_test go test ./...
