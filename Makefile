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
	cd test/e2e && go test -count=1 ./conformance/...

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
generate-licenses: go-licence-detector download-dependencies ## Regenerate NOTICE and THIRD_PARTY_LICENSES from current dependencies.
	@set -eu; \
	echo "Generating NOTICE and THIRD_PARTY_LICENSES files from current dependencies using go-licence-detector"; \
	go mod download -json > $(LOCALBIN)/deps.json; \
	$(GO_LICENCE_DETECTOR) -in $(LOCALBIN)/deps.json \
		-noticeTemplate=hack/licenses/notice.tpl \
		-noticeOut=NOTICE \
		-depsTemplate=hack/licenses/third_party_licenses.tpl \
		-depsOut=THIRD_PARTY_LICENSES; \
	echo "Done"

.PHONY: download-dependencies
download-dependencies:
	go mod download

.PHONY: check
check: download-dependencies validate test

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

##@ E2E

# Cluster name for the e2e targets. Override to run isolated clusters in parallel,
# e.g. make e2e-up CLUSTER_NAME=shard-a WORKLOADS="jobset kuberay"
CLUSTER_NAME ?= karta-e2e
# A non-default cluster gets its own kubeconfig (matching hack/e2e/up.sh) so
# parallel clusters do not race on the shared current-context.
ifneq ($(CLUSTER_NAME),karta-e2e)
E2E_KUBECONFIG := KUBECONFIG=$(HOME)/.kube/kind-$(CLUSTER_NAME).kubeconfig
endif
# Overall go-test timeout. 30m fits the full suite (all operators run in one
# suite); override for a quick subset, e.g. E2E_TIMEOUT=20m.
E2E_TIMEOUT ?= 30m

# Select cases by operator with the same WORKLOADS list as e2e-up: record-e2e WORKLOADS="nim"
# runs the nim case, WORKLOADS="jobset kuberay" runs both. Each case is labelled with its
# hack/e2e operator key (plus "builtin" for built-in kinds), and the WORKLOADS list is turned
# into a Ginkgo label filter below. E2E_LABELS overrides it with a raw label expression for set
# logic, e.g. E2E_LABELS="!builtin" or E2E_LABELS="kuberay || nim". Either composes (AND) with
# the E2E_FOCUS name regex.
empty :=
space := $(empty) $(empty)
E2E_LABELS ?= $(subst $(space), || ,$(strip $(filter-out all,$(WORKLOADS))))

# FLOW="failed" narrows a run or a record to one flow by name: it focuses the spec titled
# "<FLOW> flow ..." (see test/e2e/runner_test.go). Compose with WORKLOADS to target a single
# operator's single flow - the fast loop while adding cases, e.g.:
#   make record-e2e WORKLOADS="kuberay" FLOW="scaled"
ifneq ($(strip $(FLOW)),)
E2E_FOCUS := $(strip $(FLOW)) flow
endif

# Pick which operators to install:
#   make e2e-up                          # everything
#   make e2e-up WORKLOADS="jobset lws"   # a subset - one provision, deps resolved once
.PHONY: e2e-up
e2e-up: ## Provision a kind cluster + operators (WORKLOADS="jobset kuberay" for a subset, or "all"; CLUSTER_NAME=<name> for an isolated parallel cluster)
	CLUSTER_NAME=$(CLUSTER_NAME) ./hack/e2e/up.sh $(WORKLOADS)

.PHONY: e2e-down
e2e-down: ## Tear down the e2e cluster (set CLUSTER_NAME for a named one)
	CLUSTER_NAME=$(CLUSTER_NAME) ./hack/e2e/down.sh

.PHONY: record-e2e
record-e2e: ## Drive the e2e suite against a live cluster and record the fixtures (run e2e-up first; WORKLOADS="nim" a subset like e2e-up, FLOW="scaled" one flow, CLUSTER_NAME to match; E2E_FOCUS/E2E_LABELS for finer filters)
	cd test/e2e && go test -count=1 ./cases
	cd test/e2e && $(E2E_KUBECONFIG) go test -count=1 -v -timeout $(E2E_TIMEOUT) ./recorder $(if $(E2E_FOCUS)$(E2E_LABELS),-args $(if $(E2E_FOCUS),-ginkgo.focus="$(E2E_FOCUS)") $(if $(E2E_LABELS),-ginkgo.label-filter="$(E2E_LABELS)"))

# The e2e shell scripts to shellcheck: the provisioner, teardown, the shared
# helpers, and every per-operator install.sh/verify.sh.
E2E_SHELL := hack/e2e/up.sh hack/e2e/down.sh \
	hack/e2e/operators/_common.sh \
	$(wildcard hack/e2e/operators/*/install.sh) \
	$(wildcard hack/e2e/operators/*/verify.sh)

.PHONY: lint-shell
lint-shell: ## shellcheck the e2e shell scripts (-x follows sourced files)
	shellcheck -x $(E2E_SHELL)
