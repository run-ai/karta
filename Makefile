# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
KARTA_CHART_DIR := $(PROJECT_DIR)/charts/karta
KARTA_CRDS_DIR := $(KARTA_CHART_DIR)/crds

HELM_CHART_VERSION ?= 0.0.1

CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint
GO_LICENCE_DETECTOR ?= $(LOCALBIN)/go-licence-detector

# Tool Versions
CONTROLLER_TOOLS_VERSION ?= v0.16.5
GO_LICENCE_DETECTOR_VERSION ?= v0.10.0
# GOLANGCI_LINT_VERSION is defined in hack/tools.mk (shared with operator/Makefile).
include hack/tools.mk
PATH := $(abspath $(LOCALBIN)):$(PATH)

.PHONY: manifests
manifests: controller-gen ## Generate CRD manifests
	$(CONTROLLER_GEN) crd paths="./pkg/..." output:crd:artifacts:config=$(KARTA_CRDS_DIR)

.PHONY: generate
generate: controller-gen ## Generate DeepCopy methods
	$(CONTROLLER_GEN) object paths="./pkg/..."

.PHONY: generate-mocks
generate-mocks: ## Generate mocks using go generate
	go generate ./pkg/...

.PHONY: test
test: generate-mocks ## Run tests with mock generation
	go test ./...

lint-go: golangci-lint
	echo "Running golangci linter"
	$(GOLANGCI_LINT) run -v -c .golangci.yml
.PHONY: lint-go

fmt-go:
	go fmt ./...
.PHONY: fmt-go

vet-go:
	go vet ./...
.PHONY: vet-go

lint: fmt-go vet-go lint-go 
.PHONY: lint

.PHONY: generate-samples
generate-samples: ## Regenerate docs/catalog/ from pkg/catalog
	go run ./hack/gen-samples

.PHONY: validate
validate: generate manifests generate-mocks generate-licenses generate-samples
	@test -z "$$(git status --porcelain)" || { git status --porcelain; \
		echo "generated files are stale or untracked; run the generators and commit"; exit 1; }

.PHONY: install-crd
install-crd: manifests ## Install CRDs into the cluster
	kubectl apply --server-side -f $(KARTA_CRDS_DIR)

.PHONY: uninstall-crd
uninstall-crd: ## Uninstall CRDs from the cluster
	kubectl delete -f $(KARTA_CRDS_DIR) --ignore-not-found

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	@[ -f "$(CONTROLLER_GEN)" ] || { \
	set -e; \
	echo "Downloading controller-gen@$(CONTROLLER_TOOLS_VERSION)" ;\
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION) ;\
	}

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	@[ -f "$(GOLANGCI_LINT)" ] || { \
	set -e; \
	echo "Downloading golangci-lint@$(GOLANGCI_LINT_VERSION)" ;\
	curl -sSfL https://golangci-lint.run/install.sh  | sh -s -- -b $(LOCALBIN) $(GOLANGCI_LINT_VERSION) ;\
	}

.PHONY: go-licence-detector
go-licence-detector: $(GO_LICENCE_DETECTOR) ## Download go-licence-detector locally if necessary.
$(GO_LICENCE_DETECTOR): $(LOCALBIN)
	@[ -f "$(GO_LICENCE_DETECTOR)" ] || { \
	set -e; \
	echo "Downloading go-licence-detector@$(GO_LICENCE_DETECTOR_VERSION)" ;\
	GOBIN=$(LOCALBIN) go install go.elastic.co/go-licence-detector@$(GO_LICENCE_DETECTOR_VERSION) ;\
	}

.PHONY: generate-licenses
generate-licenses: go-licence-detector ## Regenerate NOTICE and THIRD_PARTY_LICENSES from current dependencies.
	@set -eu; \
	echo "Generating NOTICE and THIRD_PARTY_LICENSES files from current dependencies using go-licence-detector"; \
	go mod download -json > $(LOCALBIN)/root-deps.json; \
	(cd cli && go mod download -json) > $(LOCALBIN)/cli-deps.json; \
	python3 hack/merge-go-deps.py $(LOCALBIN)/root-deps.json $(LOCALBIN)/cli-deps.json > $(LOCALBIN)/deps.json; \
	$(GO_LICENCE_DETECTOR) -in $(LOCALBIN)/deps.json \
		-noticeTemplate=hack/licenses/notice.tpl \
		-noticeOut=NOTICE \
		-depsTemplate=hack/licenses/third_party_licenses.tpl \
		-depsOut=THIRD_PARTY_LICENSES; \
	echo "Done"

.PHONY: download-dependencies
download-dependencies:
	go mod download

##@ CLI

.PHONY: cli-build
cli-build: $(LOCALBIN) ## Build the karta CLI binary.
	cd cli && go build -o $(LOCALBIN)/karta .

.PHONY: cli-test
cli-test: ## Run the CLI unit tests.
	cd cli && go test ./...

.PHONY: cli-lint
cli-lint: golangci-lint ## Lint the CLI module.
	cd cli && $(GOLANGCI_LINT) run -c $(PROJECT_DIR)/.golangci.yml

.PHONY: check
check: download-dependencies validate test cli-test cli-lint

##@ Helm

.PHONY: helm-build
helm-build: ## Build the helm chart
	helm package $(KARTA_CHART_DIR) --version $(HELM_CHART_VERSION) --app-version $(HELM_CHART_VERSION)

.PHONY: helm-lint
helm-lint: ## Lint the helm chart
	helm lint $(KARTA_CHART_DIR)

# The crd-upgrader ships the CRDs in a ConfigMap, capped at ~1 MiB per etcd
# object; fail here if a growing CRD would breach it, not on a user's upgrade.
CRD_CONFIGMAP_MAX_BYTES ?= 1000000

.PHONY: helm-validate
helm-validate: ## Validate the chart renders and the CRD ConfigMap fits in etcd
	helm template $(KARTA_CHART_DIR)
	@set -e; tmp=$$(mktemp); trap 'rm -f "$$tmp"' EXIT; helm template $(KARTA_CHART_DIR) -s templates/hooks/pre/crd-upgrader-configmap.yaml > "$$tmp"; s=$$(wc -c < "$$tmp"); echo "crd-upgrader ConfigMap: $$s bytes (max $(CRD_CONFIGMAP_MAX_BYTES))"; test $$s -le $(CRD_CONFIGMAP_MAX_BYTES) || { echo "error: CRD ConfigMap is $$s bytes, over the $(CRD_CONFIGMAP_MAX_BYTES) limit; it must fit in a single ~1 MiB etcd object"; exit 1; }

##@ E2E

# Cluster name for the e2e targets. Override to run isolated clusters in parallel,
# e.g. make e2e-up CLUSTER_NAME=shard-a WORKLOADS="jobset kuberay"
CLUSTER_NAME ?= karta-e2e

# Pick which operators to install:
#   make e2e-up                          # everything
#   make e2e-up WORKLOADS="jobset lws"   # a subset - one provision, deps resolved once
.PHONY: e2e-up
e2e-up: ## Provision a kind cluster + operators (WORKLOADS="jobset kuberay" for a subset, or "all"; CLUSTER_NAME=<name> for an isolated parallel cluster)
	CLUSTER_NAME=$(CLUSTER_NAME) ./hack/e2e/up.sh $(WORKLOADS)

.PHONY: e2e-down
e2e-down: ## Tear down the e2e cluster (set CLUSTER_NAME for a named one)
	CLUSTER_NAME=$(CLUSTER_NAME) ./hack/e2e/down.sh

# The e2e shell scripts to shellcheck: the provisioner, teardown, the shared
# helpers, and every per-operator install.sh/verify.sh.
E2E_SHELL := hack/e2e/up.sh hack/e2e/down.sh \
	hack/e2e/operators/_common.sh \
	$(wildcard hack/e2e/operators/*/install.sh) \
	$(wildcard hack/e2e/operators/*/verify.sh)

.PHONY: lint-shell
lint-shell: ## shellcheck the e2e shell scripts (-x follows sourced files)
	shellcheck -x $(E2E_SHELL)
