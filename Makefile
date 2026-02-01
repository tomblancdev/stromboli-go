.PHONY: help build test test-race test-coverage test-e2e lint fmt vet generate dev shell clean

# Container settings
IMAGE_NAME := stromboli-go-dev
CONTAINER_ENGINE := podman
WORKDIR := /app

# Default target
help:
	@echo "Stromboli Go SDK - Development Commands"
	@echo ""
	@echo "Development:"
	@echo "  make dev            Run with hot reload"
	@echo "  make shell          Open container shell"
	@echo "  make build-image    Build dev container image"
	@echo ""
	@echo "Code Quality:"
	@echo "  make lint           Run golangci-lint"
	@echo "  make fmt            Format code"
	@echo "  make vet            Run go vet"
	@echo ""
	@echo "Testing:"
	@echo "  make test           Run unit tests"
	@echo "  make test-race      Run tests with race detector"
	@echo "  make test-coverage  Run tests with coverage report"
	@echo "  make test-e2e       Run E2E tests (requires Stromboli)"
	@echo ""
	@echo "Build & Generate:"
	@echo "  make build          Build binary"
	@echo "  make generate       Regenerate from OpenAPI"
	@echo ""
	@echo "Maintenance:"
	@echo "  make clean          Clean build artifacts"
	@echo "  make deps           Download dependencies"

# Build dev container image
build-image:
	$(CONTAINER_ENGINE) build -t $(IMAGE_NAME) -f Containerfile .

# Run command in container
define run_in_container
	$(CONTAINER_ENGINE) run --rm -v $(PWD):$(WORKDIR):Z -w $(WORKDIR) $(IMAGE_NAME) $(1)
endef

# Development
dev: build-image
	$(call run_in_container,go run .)

shell: build-image
	$(CONTAINER_ENGINE) run --rm -it -v $(PWD):$(WORKDIR):Z -w $(WORKDIR) $(IMAGE_NAME) /bin/sh

# Code Quality
lint: build-image
	$(call run_in_container,golangci-lint run ./...)

fmt: build-image
	$(call run_in_container,go fmt ./...)

vet: build-image
	$(call run_in_container,go vet ./...)

# Testing
test: build-image
	$(call run_in_container,go test ./...)

test-race: build-image
	$(call run_in_container,go test -race ./...)

test-coverage: build-image
	$(call run_in_container,go test -coverprofile=coverage.out ./...)
	$(call run_in_container,go tool cover -html=coverage.out -o coverage.html)
	@echo "Coverage report: coverage.html"

test-e2e: build-image
	$(call run_in_container,go test -tags=e2e ./tests/e2e/...)

# Build & Generate
build: build-image
	$(call run_in_container,go build ./...)

generate: build-image
	$(call run_in_container,go run scripts/generate.go)

# Maintenance
clean:
	rm -rf bin/ coverage.out coverage.html

deps: build-image
	$(call run_in_container,go mod download)
	$(call run_in_container,go mod tidy)
