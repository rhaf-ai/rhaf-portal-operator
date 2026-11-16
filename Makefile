.DEFAULT_GOAL := build

GO ?= go
LOCALBIN ?= $(CURDIR)/bin
LINTER_.PHONY: build run test fmt goimport fmt-check lint lint-fix vuln check clean

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

clean:
	rm -rf build dist
