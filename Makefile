.DEFAULT_GOAL := help

.PHONY: help build test vet tidy lint format ci

help: ## List available quality and build commands
	@awk 'BEGIN {FS = ":.*## "}; /^[a-zA-Z0-9_.-]+:.*## / {printf "  %-12s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the majordomo CLI
	go build ./cmd/majordomo

test: ## Run command and internal package tests
	go test ./cmd/... ./internal/...

vet: ## Run Go vet on command and internal packages
	go vet ./cmd/... ./internal/...

tidy: ## Tidy Go module dependencies
	go mod tidy

lint: ## Run golangci-lint on command and internal packages
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint is required for lint." >&2; exit 127; }
	golangci-lint run ./cmd/... ./internal/...

format: ## Check Go formatting
	@test -z "$$(gofmt -l cmd internal)" || { echo "Go files need formatting." >&2; gofmt -l cmd internal; exit 1; }

ci: ## Run format, vet, lint, tests, and build
	$(MAKE) format vet lint test build
