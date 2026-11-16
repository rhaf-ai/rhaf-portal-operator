# RHAF Portal Operator

RHAF Portal Operator will deploy and manage [RHAF Portal Core](https://github.com/rhaf-ai/rhaf-portal-core)
on Kubernetes and OpenShift. This initial Go scaffold prints `Hello, RHAF Portal Operator!`
and exits. CRDs, reconciliation, and cluster test infrastructure will follow in later tasks.

This repository owns the Go operator and its container image. Core owns the Rust
runtime. The UI and shared test infrastructure belong in separate repositories.

## Build and run

Install the Go version in `go.mod` and Make. The Go toolchain is pinned to 1.27.4,
which matches the Camel K baseline used for this scaffold.

```bash
make build
make run
make test
make fmt-check
make check
```

`make build` writes `build/rhaf-portal-operator`. `make test` runs Go tests and
`go vet`. There are no tests for the greeting. Integration and end-to-end tests
will be added with the operator implementation.

The quality settings come from Camel K: golangci-lint enables all linters with
explicit exclusions in `.golangci.yml`. Camel K command-specific exceptions are
removed. `make check` runs the linter and `govulncheck`. `make fmt` applies gofmt
and goimports formatting. Quality targets install pinned tools in the ignored
`bin/` directory on first use. They require network access.

## Container image

The multi-stage `Containerfile` builds a static Go binary and runs it on Fedora
Minimal 44 as UID 1001 with GID 0. OpenShift can assign a different UID. The
bootstrap image prints the greeting and exits.

Install Podman or Docker. Build and run the native image:

```bash
make build-image IMAGE_TAG=localhost/rhaf-portal-operator:dev
podman run --rm localhost/rhaf-portal-operator:dev-arm64
```

Use `-amd64` on Intel and AMD hosts. Set `CONTAINER_TOOL=docker` on Make commands
to use Docker. Use the same tool to build and run the image.

To publish a multi-architecture image, authenticate with the registry. Run the
first command on both Linux amd64 and Linux arm64 hosts. Run the manifest command
once after both images are available:

```bash
make build-image PUSH=true IMAGE_TAG=REGISTRY/GROUP/rhaf-portal-operator:TAG
make build-image-manifest IMAGE_TAG=REGISTRY/GROUP/rhaf-portal-operator:TAG
```

The build adds an architecture suffix. The manifest publishes the combined tag.
The manifest command supports Podman and Docker.

## CI and releases

- `pr-build`: Go build, tests, formatting, lint, vulnerability scan, and container builds.
- `main-build`: the same Go checks, plus native Linux amd64 and arm64 images and a combined manifest.
- `lychee-links`: Markdown links and formatting.
- `early-access`: manually publish four-platform archives and rolling prereleases.
- `release`: publish versioned archives and images from a version tag or manual run.

Container publication uses organization variables `CONTAINER_REGISTRY` and
`CONTAINER_GROUP`, plus secrets `QUAY_USERNAME` and `QUAY_PASSWORD`. Configure all
four before running a publishing workflow. GitHub releases use the workflow's
`GITHUB_TOKEN`. Release publication waits for container manifests.

Stable release versions use `MAJOR.MINOR.PATCH`. Push a tag such as `v0.1.0`, or
run `release` manually with version `0.1.0`. Run `early-access` manually with
`currentDevelopmentVersion`, such as `0.1.0-dev`.

Release workflows package Linux and macOS binaries for amd64 and arm64. Each
archive includes the binary, README, and license. The release includes SHA-256
checksums. To create an archive locally:

```bash
make release-archive VERSION=0.1.0 PLATFORM=linux-x86_64
```

Supported platform names are `linux-x86_64`, `linux-aarch64`, `macos-x86_64`, and
`macos-aarch64`. Archives are written to `dist/`.

Dependabot checks Go modules, GitHub Actions, and container base images weekly.

## License

[Apache License 2.0](LICENSE).
