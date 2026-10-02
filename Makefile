GO ?= go
GOBIN := $(shell $(GO) env GOPATH)/bin
GOLANGCI_LINT_VERSION := v2.14.0
GOLANGCI_LINT ?= $(GOBIN)/golangci-lint
NPM ?= npm
DOCKER_COMPOSE ?= docker compose
# The Docker test database (service postgres-test). make test never loads .env.
TEST_DATABASE_URL ?= postgres://postgres:postgres@127.0.0.1:5433/expense_test?sslmode=disable
MIGRATIONS_DIR := ../db/migrations

.DEFAULT_GOAL := help
.PHONY: help tools test lint run migrate migrate-status migrate-down db-login-password web-install web-dev

help: ## List the targets
	@grep -E '^[a-z][a-z-]*:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "  %-18s %s\n", $$1, $$2}'

tools: ## Install golangci-lint (pinned version, sha256-checked) into GOPATH/bin
	@set -eu; \
	v=$(GOLANGCI_LINT_VERSION); ver=$${v#v}; \
	name=golangci-lint-$$ver-$$($(GO) env GOOS)-$$($(GO) env GOARCH); \
	base=https://github.com/golangci/golangci-lint/releases/download/$$v; \
	tmp=$$(mktemp -d); trap 'rm -rf "$$tmp"' EXIT; \
	curl -sSfL -o "$$tmp/$$name.tar.gz" "$$base/$$name.tar.gz"; \
	curl -sSfL -o "$$tmp/checksums.txt" "$$base/golangci-lint-$$ver-checksums.txt"; \
	(cd "$$tmp" && grep " $$name.tar.gz\$$" checksums.txt | sha256sum -c -); \
	tar -xzf "$$tmp/$$name.tar.gz" -C "$$tmp"; \
	mkdir -p "$(GOBIN)"; \
	install -m 0755 "$$tmp/$$name/golangci-lint" "$(GOBIN)/golangci-lint"; \
	"$(GOBIN)/golangci-lint" version

# The API tests apply the migrations to the test database themselves (internal/db/dbtest).
# -count=1: Go's test cache does not see changes in db/migrations or in the database.
test: ## Start postgres-test, run the API tests on it, then the web tests
	$(DOCKER_COMPOSE) up -d --wait postgres-test
	cd api && TEST_DATABASE_URL='$(TEST_DATABASE_URL)' $(GO) test -count=1 ./...
	cd web && $(NPM) test

lint: ## Lint the API, then lint, format-check and type-check the web
	cd api && $(GOLANGCI_LINT) run ./...
	cd web && $(NPM) run lint
	cd web && $(NPM) run format:check
	cd web && $(NPM) run typecheck

run: ## Run the API as app_login from DATABASE_URL (loads .env if it exists)
	@set -a; if [ -f ./.env ]; then . ./.env; fi; set +a; \
	cd api && $(GO) run ./cmd/api

migrate: ## Apply all pending migrations as the owner, MIGRATION_DATABASE_URL (loads .env)
	@set -a; if [ -f ./.env ]; then . ./.env; fi; set +a; \
	cd api && $(GO) run ./cmd/migrate -dir $(MIGRATIONS_DIR) up

migrate-status: ## List the migrations and whether each is applied (MIGRATION_DATABASE_URL; loads .env)
	@set -a; if [ -f ./.env ]; then . ./.env; fi; set +a; \
	cd api && $(GO) run ./cmd/migrate -dir $(MIGRATIONS_DIR) status

migrate-down: ## Roll back the latest migration (MIGRATION_DATABASE_URL); needs CONFIRM=yes (loads .env)
	@if [ "$(CONFIRM)" != "yes" ]; then \
		echo "migrate-down rolls back the latest migration and can delete data."; \
		echo "Run: make migrate-down CONFIRM=yes"; \
		exit 1; \
	fi; \
	set -a; if [ -f ./.env ]; then . ./.env; fi; set +a; \
	cd api && $(GO) run ./cmd/migrate -dir $(MIGRATIONS_DIR) down

# Run once after the first `make migrate`, and again only to change the password: each run
# replaces it, so the old DATABASE_URL line stops working.
db-login-password: ## Set a new random password on app_login and print the DATABASE_URL line (loads .env)
	@set -a; if [ -f ./.env ]; then . ./.env; fi; set +a; \
	cd api && $(GO) run ./cmd/migrate login-password

web-install: ## Install the web dependencies from package-lock.json
	cd web && $(NPM) ci

# Only API_ADDR from .env is passed on: the dev server needs nothing else.
web-dev: ## Run the web dev server (proxies /api to API_ADDR)
	@if [ -f ./.env ]; then . ./.env; fi; \
	cd web && API_ADDR="$${API_ADDR:-}" $(NPM) run dev
