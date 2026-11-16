.DEFAULT_GOAL := build

GO ?= go

.PHONY: build run test clean

build:
	mkdir -p build
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags='-s -w' -o build/rhaf-portal-operator ./cmd/rhaf-portal-operator

run:
	$(GO) run ./cmd/rhaf-portal-operator

test:
	$(GO) test ./...
	$(GO) vet ./...

clean:
	rm -rf build dist
