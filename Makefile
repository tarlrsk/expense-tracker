GO ?= go
GOBIN := $(shell $(GO) env GOPATH)/bin
GOLANGCI_LINT_VERSION := v2.14.0
GOLANGCI_LINT ?= $(GOBIN)/golangci-lint

.DEFAULT_GOAL := help
.PHONY: help tools test lint run

help: ## List the targets
	@grep -E '^[a-z][a-z-]*:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "  %-8s %s\n", $$1, $$2}'

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

# Starting postgres-test and applying the migrations is added in PLAN-0002 T3.
test: ## Run the API tests
	cd api && $(GO) test ./...

lint: ## Run golangci-lint on the API
	cd api && $(GOLANGCI_LINT) run ./...

run: ## Run the API (loads .env if it exists)
	@set -a; if [ -f ./.env ]; then . ./.env; fi; set +a; \
	cd api && $(GO) run ./cmd/api
