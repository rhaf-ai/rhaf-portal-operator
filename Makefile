.DEFAULT_GOAL := build

GO ?= go
LOCALBIN ?= $(CURDIR)/bin
LINTER_VERSION ?= v2.14.0
GOVULNCHECK_VERSION ?= v1.8.0
GOIMPORTS_VERSION ?= v0.50.0
LINTER := $(LOCALBIN)/golangci-lint-$(LINTER_VERSION)
GOVULNCHECK := $(LOCALBIN)/govulncheck-$(GOVULNCHECK_VERSION)
GOIMPORT := $(LOCALBIN)/goimports-$(GOIMPORTS_VERSION)
LINT_GOGC ?= 20
LINT_DEADLINE ?= 10m

VERSION ?= 0.1.0-dev
PLATFORM ?= $(shell $(GO) env GOOS)-$(shell $(GO) env GOARCH)
CONTAINER_TOOL ?= podman
CONTAINER_REGISTRY ?= quay.io
CONTAINER_GROUP ?= rhaf-ai
IMAGE_TAG ?= $(CONTAINER_REGISTRY)/$(CONTAINER_GROUP)/rhaf-portal-operator:$(VERSION)
PUSH ?= false

.PHONY: build run test fmt goimport fmt-check lint lint-fix vuln check clean build-image build-image-manifest release-archive

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

# Native builds use the same architecture suffixes as the published manifest.
build-image:
	CONTAINER_TOOL='$(CONTAINER_TOOL)' IMAGE_TAG='$(IMAGE_TAG)' PUSH='$(PUSH)' ./hack/build-image.sh

build-image-manifest:
	CONTAINER_TOOL='$(CONTAINER_TOOL)' IMAGE_TAG='$(IMAGE_TAG)' ./hack/build-image-manifest.sh

release-archive:
	GO='$(GO)' VERSION='$(VERSION)' PLATFORM='$(PLATFORM)' ./hack/release-archive.sh

clean:
	rm -rf build dist
