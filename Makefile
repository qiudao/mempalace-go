.DEFAULT_GOAL := help

.PHONY: help build test lint

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

build: ## Build mempalace-go binary
	go build -o bin/mempalace ./cmd/mempalace

test: ## Run all tests
	go test ./... -v -count=1

lint: ## Run go vet
	go vet ./...
