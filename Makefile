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
RI_CHART_DIR := $(PROJECT_DIR)/charts/ri
RI_CRDS_DIR := $(RI_CHART_DIR)/crds

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

.PHONY: manifests
manifests: controller-gen ## Generate CRD manifests
	$(CONTROLLER_GEN) crd paths="./pkg/..." output:crd:artifacts:config=$(RI_CRDS_DIR)

.PHONY: generate
generate: controller-gen ## Generate DeepCopy methods
	$(CONTROLLER_GEN) object paths="./..."

.PHONY: generate-mocks
generate-mocks: mockgen ## Generate mocks using go generate
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

.PHONY: validate
validate: generate manifests generate-mocks generate-licenses
	@git diff --exit-code 

.PHONY: install-crd
install-crd: manifests ## Install CRDs into the cluster
	kubectl apply --server-side -f $(RI_CRDS_DIR)

.PHONY: uninstall-crd
uninstall-crd: ## Uninstall CRDs from the cluster
	kubectl delete -f $(RI_CRDS_DIR) --ignore-not-found

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
