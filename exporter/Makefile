# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation

# Location to install dependencies to, and to output built binaries.
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

# Version is derived automatically from git. Falls back to "0.0.0-main" when
# there are no tags or git is unavailable (e.g. a bare source tarball).
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.0.0-main")

# Container image settings (defaults for ghcr.io/run-ai/karta OSS publishing).
IMAGE_REGISTRY ?= ghcr.io/run-ai/karta
IMAGE_NAME      ?= karta-exporter
IMAGE_TAG       ?= $(VERSION)
IMAGE           ?= $(IMAGE_REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)

CONTAINER_TOOL ?= docker
BUILD_ARGS     ?=

VERSION_PKG = github.com/run-ai/karta/pkg/version
LDFLAGS     = -ldflags "-X $(VERSION_PKG).version=$(VERSION)"

GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint
# GOLANGCI_LINT_VERSION is defined in ../hack/tools.mk (shared with the root
# Makefile) so the linter version has a single source of truth.
include ../hack/tools.mk

##@ Development

.PHONY: fmt
fmt: ## Run go fmt (fixes files in place)
	go fmt ./...

.PHONY: fmt-check
fmt-check: ## Check formatting without modifying files (intended for CI)
	@set -e; \
	dirs="$$(go list -f '{{.Dir}}' ./...)"; \
	unformatted="$$(gofmt -l $$dirs)"; \
	test -z "$$unformatted" || (echo "go fmt required:"; echo "$$unformatted"; exit 1)

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: lint
lint: golangci-lint ## Run golangci-lint (set VERBOSE=1 for -v)
	$(GOLANGCI_LINT) run $(if $(VERBOSE),-v) -c ../.golangci.yml

.PHONY: test
test: ## Run unit tests
	go test -race -coverprofile=cover.out ./...

.PHONY: check
check: fmt-check vet lint test ## Run all checks (fmt-check, vet, lint, test)

##@ Build

.PHONY: build
build: ## Build the karta-exporter binary for the host OS/arch
	mkdir -p $(LOCALBIN)
	go build -trimpath $(LDFLAGS) -o $(LOCALBIN)/karta-exporter ./cmd

##@ Docker

.PHONY: docker-build
docker-build: ## Build Docker image for the host arch
	$(CONTAINER_TOOL) build $(BUILD_ARGS) \
		--build-arg TARGETARCH=$(shell go env GOARCH) \
		--build-arg VERSION=$(VERSION) \
		--tag $(IMAGE) \
		-f Dockerfile \
		..

.PHONY: docker-push
docker-push: ## Push the Docker image
	$(CONTAINER_TOOL) push $(IMAGE)

##@ Tools

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	@[ -f "$(GOLANGCI_LINT)" ] || { \
	set -e; \
	echo "Downloading golangci-lint@$(GOLANGCI_LINT_VERSION)"; \
	curl -sSfL https://golangci-lint.run/install.sh \
		| sh -s -- -b $(LOCALBIN) $(GOLANGCI_LINT_VERSION); \
	}

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} \
	/^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-25s\033[0m %s\n", $$1, $$2 } \
	/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help
