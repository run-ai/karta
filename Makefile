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

# Tool Binaries
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
MOCKGEN ?= $(LOCALBIN)/mockgen

# Tool Versions
CONTROLLER_TOOLS_VERSION ?= v0.16.5
GOMOCK_VERSION ?= v0.6.0

PATH := $(abspath $(LOCALBIN)):$(PATH)

.PHONY: manifests
manifests: controller-gen ## Generate CRD manifests
	$(CONTROLLER_GEN) crd paths="./..." output:crd:artifacts:config=config/crd

.PHONY: generate
generate: controller-gen ## Generate DeepCopy methods
	$(CONTROLLER_GEN) object paths="./..."

.PHONY: generate-mocks
generate-mocks: mockgen ## Generate mocks using go generate
	go generate ./pkg/...

.PHONY: test
test: generate-mocks ## Run tests with mock generation
	go test ./...

lint-go:
	echo "Running golangci linter"
	golangci-lint run -v -c .golangci.yml
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
validate: generate manifests generate-mocks lint 
	@git diff --exit-code 

.PHONY: install-crd
install-crd: manifests ## Install CRDs into the cluster
	kubectl apply --server-side -f config/crd/

.PHONY: uninstall-crd
uninstall-crd: ## Uninstall CRDs from the cluster
	kubectl delete -f config/crd/ --ignore-not-found

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
