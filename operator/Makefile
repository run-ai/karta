ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

# Build output directory for binaries (expected by Dockerfile)
BIN_DIR ?= $(shell pwd)/bin

# Version is derived automatically from git. Falls back to "dev" when there
# are no tags or git is unavailable (e.g. a bare source tarball).
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")


# VERSION_PKG is the Go package path that holds the Version variable. The build
# targets inject VERSION into the binary via -ldflags so it is available at runtime.
VERSION_PKG = github.com/run-ai/karta/operator/pkg/version
LDFLAGS     = -ldflags "-X $(VERSION_PKG).Version=$(VERSION)"

GOLANGCI_LINT         ?= $(LOCALBIN)/golangci-lint
GOLANGCI_LINT_VERSION ?= v2.5.0

ENVTEST          ?= $(LOCALBIN)/setup-envtest
ENVTEST_VERSION  ?= release-0.23
# Kubernetes control-plane (apiserver/etcd/kubectl) version envtest downloads.
ENVTEST_K8S_VERSION ?= 1.31.0

# Architectures to build for `make build-all`
PLATFORMS ?= linux/amd64 linux/arm64

##@ Development

.PHONY: fmt
fmt: ## Run go fmt (fixes files in place)
	go fmt ./...

.PHONY: fmt-check
fmt-check: ## Check formatting without modifying files (intended for CI)
	@test -z "$(shell gofmt -l .)" || (echo "go fmt required:"; gofmt -l .; exit 1)

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: lint
lint: golangci-lint ## Run golangci-lint
	$(GOLANGCI_LINT) run -v -c ../.golangci.yml

.PHONY: test-unit
test-unit: ## Run unit tests (no envtest binaries required)
	go test -coverprofile=cover-unit.out ./pkg/... ./cmd/...

.PHONY: test-integration
test-integration: envtest ## Run envtest integration tests (downloads control-plane binaries)
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" \
		go test -coverprofile=cover-integration.out ./test/integration/...

.PHONY: test
test: test-unit test-integration ## Run unit + integration tests

.PHONY: check
check: fmt-check vet lint test ## Run all checks (fmt-check, vet, lint, test)

##@ Build

.PHONY: build
build: ## Build the karta-operator binary for the host OS/arch
	go build $(LDFLAGS) -o $(BIN_DIR)/karta-operator ./cmd

.PHONY: build-linux-amd64
build-linux-amd64: ## Cross-compile for linux/amd64
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BIN_DIR)/karta-operator-amd64 ./cmd

.PHONY: build-linux-arm64
build-linux-arm64: ## Cross-compile for linux/arm64
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BIN_DIR)/karta-operator-arm64 ./cmd

.PHONY: build-all
build-all: build-linux-amd64 build-linux-arm64 ## Cross-compile for all supported platforms

##@ Docker

.PHONY: docker-build
docker-build: build-linux-amd64 ## Build Docker image for amd64
	docker build \
		--build-arg TARGETARCH=amd64 \
		--build-arg VERSION=$(VERSION) \
		--tag $(IMAGE) \
		.

.PHONY: docker-build-multiarch
docker-build-multiarch: build-all ## Build and push multi-arch Docker image via BuildKit
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		--build-arg VERSION=$(VERSION) \
		--tag $(IMAGE) \
		--push \
		.

.PHONY: docker-push
docker-push: ## Push the Docker image
	docker push $(IMAGE)

##@ Kubernetes

.PHONY: deploy
deploy: ## Deploy the operator to the cluster in the current kubeconfig context
	kubectl apply -f config/

.PHONY: undeploy
undeploy: ## Remove the operator from the cluster
	kubectl delete -f config/ --ignore-not-found

##@ Tools

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	@[ -f "$(ENVTEST)" ] || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@$(ENVTEST_VERSION)

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	@[ -f "$(GOLANGCI_LINT)" ] || { \
	set -e; \
	echo "Downloading golangci-lint@$(GOLANGCI_LINT_VERSION)"; \
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh \
		| sh -s -- -b $(LOCALBIN) $(GOLANGCI_LINT_VERSION); \
	}

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} \
	/^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-25s\033[0m %s\n", $$1, $$2 } \
	/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help
