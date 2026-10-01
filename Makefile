.DEFAULT_GOAL := help

.PHONY: help build test vet tidy lint format

help: ## List available make verbs
	@awk 'BEGIN {FS = ":.*## "}; /^[a-zA-Z0-9_-]+:.*## / {printf "  %-12s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the majordomo CLI
	go build ./cmd/majordomo

test: ## Run all Go tests
	go test ./...

vet: ## Run Go vet
	go vet ./...

tidy: ## Tidy Go module dependencies
	go mod tidy

lint: ## Run golangci-lint
	GOTOOLCHAIN=go1.27.0 go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./cmd/... ./internal/...

format: ## Format Go source files
	gofmt -w cmd internal pkg
