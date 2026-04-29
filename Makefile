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
MOCKGEN ?= $(LOCALBIN)/mockgen
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint
GO_LICENSES ?= $(LOCALBIN)/go-licenses
GOROOT ?= $(shell go env GOROOT)
# Tool Versions
CONTROLLER_TOOLS_VERSION ?= v0.16.5
GOMOCK_VERSION ?= v0.6.0
GOLANGCI_LINT_VERSION ?= v2.5.0
GO_LICENSES_VERSION ?= v2.0.1
PATH := $(abspath $(LOCALBIN)):$(PATH)

# --- Multi-module discovery ------------------------------------------------
# Every Go module in this repo (sorted, vendor/bin/.git excluded). The repo
# is split across sub-modules so consumers can pull a minimal dep surface;
# tools that operate on "this module" (go test, golangci-lint, go mod tidy,
# controller-gen, go generate) must therefore be run once per module — `./...`
# stops at module boundaries and does not cross go.mod files.
GO_MOD_DIRS := $(shell find . \( -path ./vendor -o -path ./bin -o -path ./.git \) -prune -false -o -name go.mod -exec dirname {} \; | sort)

# Modules that ship Kubernetes CRD types (have +kubebuilder markers). Listed
# explicitly because controller-gen output paths are deliberate (boilerplate
# header, CRD output dir). Append the dir here if you add a new CRD-bearing
# module.
CRD_MODULE_DIRS := pkg/api/runai/v1alpha1

.PHONY: manifests
manifests: controller-gen ## Generate CRD manifests across every CRD-bearing sub-module
	@for dir in $(CRD_MODULE_DIRS); do \
	  echo ">> manifests in $$dir"; \
	  (cd $$dir && $(CONTROLLER_GEN) crd paths="./..." output:crd:artifacts:config=$(KARTA_CRDS_DIR)) || exit 1; \
	done

.PHONY: generate
generate: controller-gen ## Regenerate zz_generated.deepcopy.go in every CRD-bearing sub-module
	@for dir in $(CRD_MODULE_DIRS); do \
	  echo ">> deepcopy in $$dir"; \
	  (cd $$dir && $(CONTROLLER_GEN) object paths="./...") || exit 1; \
	done

.PHONY: generate-mocks
generate-mocks: mockgen ## Run go generate (mockgen) across every sub-module
	@for dir in $(GO_MOD_DIRS); do \
	  echo ">> go generate in $$dir"; \
	  (cd $$dir && go generate ./...) || exit 1; \
	done

.PHONY: test
test: generate-mocks ## Run tests across every sub-module
	@for dir in $(GO_MOD_DIRS); do \
	  echo ">> go test in $$dir"; \
	  (cd $$dir && go test ./...) || exit 1; \
	done

.PHONY: tidy
tidy: ## go mod tidy across every sub-module
	@for dir in $(GO_MOD_DIRS); do \
	  echo ">> go mod tidy in $$dir"; \
	  (cd $$dir && go mod tidy) || exit 1; \
	done

lint-go: golangci-lint
	@for dir in $(GO_MOD_DIRS); do \
	  echo ">> golangci-lint in $$dir"; \
	  (cd $$dir && $(GOLANGCI_LINT) run -v -c $(PROJECT_DIR)/.golangci.yml) || exit 1; \
	done
.PHONY: lint-go

fmt-go:
	@for dir in $(GO_MOD_DIRS); do \
	  echo ">> go fmt in $$dir"; \
	  (cd $$dir && go fmt ./...) || exit 1; \
	done
.PHONY: fmt-go

vet-go:
	@for dir in $(GO_MOD_DIRS); do \
	  echo ">> go vet in $$dir"; \
	  (cd $$dir && go vet ./...) || exit 1; \
	done
.PHONY: vet-go

lint: fmt-go vet-go lint-go
.PHONY: lint

.PHONY: validate
validate: generate manifests generate-mocks generate-licenses
	@git diff --exit-code 

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

.PHONY: mockgen
mockgen: $(MOCKGEN) ## Download mockgen locally if necessary.
$(MOCKGEN): $(LOCALBIN)
	@[ -f "$(MOCKGEN)" ] || { \
	set -e; \
	echo "Downloading mockgen@$(GOMOCK_VERSION)" ;\
	GOBIN=$(LOCALBIN) go install go.uber.org/mock/mockgen@$(GOMOCK_VERSION) ;\
	}

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	@[ -f "$(GOLANGCI_LINT)" ] || { \
	set -e; \
	echo "Downloading golangci-lint@$(GOLANGCI_LINT_VERSION)" ;\
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(LOCALBIN) $(GOLANGCI_LINT_VERSION) ;\
	}

.PHONY: go-licenses
go-licenses: $(GO_LICENSES) ## Download go-licenses locally if necessary.
$(GO_LICENSES): $(LOCALBIN)
	@[ -f "$(GO_LICENSES)" ] || { \
	set -e; \
	echo "Downloading go-licenses@$(GO_LICENSES_VERSION)" ;\
	GOBIN=$(LOCALBIN) go install github.com/google/go-licenses/v2@$(GO_LICENSES_VERSION) ;\
	}

.PHONY: generate-licenses
generate-licenses: go-licenses download-dependencies ## Regenerate NOTICE and THIRD_PARTY_LICENSES from current dependencies.
	echo "Updating NOTICE and THIRD_PARTY_LICENSES"
	`@set` -e; \
	tmp_notice=$$(mktemp); \
	tmp_third=$$(mktemp); \
	GOROOT=$(GOROOT) $(GO_LICENSES) report ./... --ignore github.com/run-ai/karta --template=hack/licenses/notice.tpl > $$tmp_notice; \
	GOROOT=$(GOROOT) $(GO_LICENSES) report ./... --ignore github.com/run-ai/karta --template=hack/licenses/third_party_licenses.tpl > $$tmp_third; \
	mv $$tmp_notice NOTICE; \
	mv $$tmp_third THIRD_PARTY_LICENSES

.PHONY: download-dependencies
download-dependencies:
	go mod download

.PHONY: check
check: download-dependencies validate test lint

##@ Helm

.PHONY: helm-build
helm-build: ## Build the helm chart
	helm package $(KARTA_CHART_DIR) --version $(HELM_CHART_VERSION) --app-version $(HELM_CHART_VERSION)

.PHONY: helm-lint
helm-lint: ## Lint the helm chart
	helm lint $(KARTA_CHART_DIR)

.PHONY: helm-validate
helm-validate: ## Validate the helm chart renders
	helm template $(KARTA_CHART_DIR)
