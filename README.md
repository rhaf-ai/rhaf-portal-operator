# RHAF Portal Operator

RHAF Portal Operator manages [RHAF Portal Core](https://github.com/rhaf-ai/rhaf-portal-core)
on Kubernetes and OpenShift. It defines namespaced installation and MCP endpoint
resources, and starts controllers that maintain their finalizers. Core workload
and endpoint registration reconciliation follow in later tasks.

This repository owns the Go operator and its container image. Core owns the Rust
runtime. The UI and shared test infrastructure belong in separate repositories.

## Build and run

Install the Go version in `go.mod` and Make. The Go toolchain is pinned to 1.27.4,
which matches the Camel K baseline used for this scaffold.

```bash
make build
make test
make fmt-check
make check
```

`make build` writes `build/rhaf-portal-operator`. `make test` runs Go tests and
`go vet`. Integration and end-to-end tests will be added with workload
reconciliation.

Generate the API artifacts after changing API markers or types:

```bash
make generate
make manifests
```

Generated CRDs are stored in `config/crd/bases`. Both `RHAFPortalCore` and
`RHAFMCPServer` are namespaced resources in `rhaf.ai.redhat.com/v1alpha1` and
provide a `status` subresource. An MCP server declaration identifies the Core
forward with `metadata.name`; it declares an existing endpoint and does not
install a workload. `spec.namespaceBase` accepts 1–51 lowercase letters, digits,
or hyphens, without an edge hyphen. Its status conditions distinguish
configuration validity, registration, freshness, upstream availability,
presentation readiness, and optional OLS readiness.

The quality settings come from Camel K: golangci-lint enables all linters with
explicit exclusions in `.golangci.yml`. Camel K command-specific exceptions are
removed. `make check` runs the linter and `govulncheck`. `make fmt` applies gofmt
and goimports formatting. Quality targets install pinned tools in the ignored
`bin/` directory on first use. They require network access.

## Development clusters

The operator runs with the service-account credentials supplied by a Kubernetes
or OpenShift cluster. It reconciles finalizers for `RHAFPortalCore` and
`RHAFMCPServer`, serves metrics on port 8080, and serves health probes on port
8081.

For Minikube development, start the cluster and deploy the Helm chart:

```bash
minikube start
make dev-minikube
```

For OpenShift Local development, publish an operator image to the OpenShift
registry, then deploy the Helm chart with its repository and tag:

```bash
crc start
eval "$(crc oc-env)"
oc login -u kubeadmin -p '<CRC password>'
make dev-openshift-local \
  HELM_NAMESPACE=rhaf-system \
  HELM_IMAGE='<OpenShift registry>/rhaf-system/rhaf-portal-operator:dev'
```

Both targets install the CRDs, namespace-scoped RBAC, and a single manager
Deployment through the chart in `charts/rhaf-portal-operator`. The packaged
installation bundle is tracked in issue #8.

Manifest publication requires Podman, including when Docker builds the images.
Authenticate the build tool with the registry before pushing architecture images.
Run the build command on both Linux amd64 and Linux arm64 hosts. Then authenticate
Podman and run the manifest command once after both images are available:

```bash
make build-image PUSH=true IMAGE_TAG=REGISTRY/GROUP/rhaf-portal-operator:TAG
podman login REGISTRY
make build-image-manifest IMAGE_TAG=REGISTRY/GROUP/rhaf-portal-operator:TAG
```

The build adds an architecture suffix. The manifest publishes the combined tag.
Podman combines all entries from the architecture tags, including tags that are
themselves image indexes.

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
