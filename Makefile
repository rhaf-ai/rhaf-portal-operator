.DEFAULT_GOAL := build

GO ?= go
LOCALBIN ?= $(CURDIR)/bin
LINTER_VERSION ?= v2.14.0
GOVULNCHECK_VERSION ?= v1.8.0
GOIMPORTS_VERSION ?= v0.50.0
CONTROLLER_TOOLS_VERSION ?= v0.19.0
LINTER := $(LOCALBIN)/golangci-lint-$(LINTER_VERSION)
GOVULNCHECK := $(LOCALBIN)/govulncheck-$(GOVULNCHECK_VERSION)
GOIMPORT := $(LOCALBIN)/goimports-$(GOIMPORTS_VERSION)
CONTROLLER_GEN := $(LOCALBIN)/controller-gen-$(CONTROLLER_TOOLS_VERSION)
HELM ?= helm
HELM_CHART ?= charts/rhaf-portal-operator
HELM_RELEASE ?= rhaf-portal-operator
HELM_NAMESPACE ?= rhaf-system
HELM_IMAGE ?= quay.io/rhaf-ai/rhaf-portal-operator:dev
HELM_IMAGE_PULL_POLICY ?= IfNotPresent
LINT_GOGC ?= 20
LINT_DEADLINE ?= 10m

VERSION ?= 0.1.0-dev
PLATFORM ?= $(shell $(GO) env GOOS)-$(shell $(GO) env GOARCH)
CONTAINER_TOOL ?= podman
CONTAINER_REGISTRY ?= quay.io
CONTAINER_GROUP ?= rhaf-ai
IMAGE_TAG ?= $(CONTAINER_REGISTRY)/$(CONTAINER_GROUP)/rhaf-portal-operator:$(VERSION)
PUSH ?= false

.PHONY: build run test fmt goimport fmt-check lint lint-fix vuln check generate manifests helm-lint dev-minikube dev-openshift-local clean build-image build-image-manifest test-image-manifest release-archive

build:
	mkdir -p build
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags='-s -w' -o build/rhaf-portal-operator ./cmd/rhaf-portal-operator

run:
	$(GO) run ./cmd/rhaf-portal-operator

test:
	$(GO) test ./...
	$(GO) vet ./...

fmt goimport: $(GOIMPORT)
	$(GOIMPORT) -w .

fmt-check: $(GOIMPORT)
	@files=$$($(GOIMPORT) -l .) && test -z "$$files" || { echo 'Run make fmt to format Go files.' >&2; exit 1; }

lint: $(LINTER)
	GOGC=$(LINT_GOGC) $(LINTER) run --config .golangci.yml --timeout $(LINT_DEADLINE)

lint-fix: $(LINTER)
	GOGC=$(LINT_GOGC) $(LINTER) run --config .golangci.yml --timeout $(LINT_DEADLINE) --fix

vuln: $(GOVULNCHECK)
	$(GOVULNCHECK) ./...

check: lint vuln

# generate creates DeepCopy implementations for the API types.
generate: $(CONTROLLER_GEN) $(GOIMPORT)
	$(CONTROLLER_GEN) object:headerFile="" paths="./api/..."
	$(GOIMPORT) -w ./api

# manifests generates CRDs and RBAC manifests from Kubebuilder markers.
manifests: $(CONTROLLER_GEN)
	$(CONTROLLER_GEN) crd paths="./api/..." output:crd:artifacts:config=config/crd/bases
	$(CONTROLLER_GEN) rbac:roleName=manager-role paths="./controllers/..." output:rbac:artifacts:config=config/rbac
	cp config/crd/bases/*.yaml $(HELM_CHART)/crds/

helm-lint:
	$(HELM) lint $(HELM_CHART)

dev-minikube:
	minikube image build -t $(HELM_IMAGE) -f Containerfile .
	$(HELM) upgrade --install $(HELM_RELEASE) $(HELM_CHART) --namespace $(HELM_NAMESPACE) --create-namespace \
		--set image='$(HELM_IMAGE)' --set imagePullPolicy=$(HELM_IMAGE_PULL_POLICY)

dev-openshift-local:
	$(HELM) upgrade --install $(HELM_RELEASE) $(HELM_CHART) --namespace $(HELM_NAMESPACE) --create-namespace \
		--set image='$(HELM_IMAGE)' --set imagePullPolicy=Always

$(LINTER):
	mkdir -p $(LOCALBIN)
	GOBIN=$(LOCALBIN) $(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(LINTER_VERSION)
	mv $(LOCALBIN)/golangci-lint $@

$(GOVULNCHECK):
	mkdir -p $(LOCALBIN)
	GOBIN=$(LOCALBIN) $(GO) install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
	mv $(LOCALBIN)/govulncheck $@

$(GOIMPORT):
	mkdir -p $(LOCALBIN)
	GOBIN=$(LOCALBIN) $(GO) install golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION)
	mv $(LOCALBIN)/goimports $@

$(CONTROLLER_GEN):
	mkdir -p $(LOCALBIN)
	GOBIN=$(LOCALBIN) $(GO) install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)
	mv $(LOCALBIN)/controller-gen $@

# Native builds use the same architecture suffixes as the published manifest.
build-image:
	CONTAINER_TOOL='$(CONTAINER_TOOL)' IMAGE_TAG='$(IMAGE_TAG)' PUSH='$(PUSH)' ./scripts/build-image.sh

build-image-manifest:
	CONTAINER_TOOL='$(CONTAINER_TOOL)' IMAGE_TAG='$(IMAGE_TAG)' ./scripts/build-image-manifest.sh

test-image-manifest:
	./scripts/test-build-image-manifest.sh

release-archive:
	GO='$(GO)' VERSION='$(VERSION)' PLATFORM='$(PLATFORM)' ./scripts/release-archive.sh

clean:
	rm -rf build dist
