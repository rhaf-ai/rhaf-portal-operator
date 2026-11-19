# RHAF Portal Operator

RHAF Portal Operator manages [RHAF Portal Core](https://github.com/rhaf-ai/rhaf-portal-core)
on Kubernetes and OpenShift. It defines namespaced installation and MCP endpoint
resources. The installation controller runs the Core workload. Endpoint
registration reconciliation follows in a later task.

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

## Install Core

Helm installs the CRDs only on the first installation and does not upgrade them.
Before you upgrade an existing installation, apply the current CRDs so that
`RHAFPortalCore` gets the new fields and validation:

```bash
kubectl apply --server-side -f config/crd/bases/
helm upgrade rhaf-portal-operator charts/rhaf-portal-operator --namespace rhaf-system
```

To use a Wanaku bootstrap configuration, create the ConfigMap first:

```bash
kubectl create configmap wanaku --from-file=wanaku.yaml=/path/to/wanaku.yaml
```

Create one `RHAFPortalCore` in the namespace that runs Core:

```yaml
apiVersion: rhaf.ai.redhat.com/v1alpha1
kind: RHAFPortalCore
metadata:
  name: core
spec:
  image: quay.io/rhaf-ai/rhaf-portal-core:latest
  storage:
    size: 100Mi          # default
  management:
    port: 8080           # default
  mcp:
    port: 8081           # default
  wanakuConfig:          # optional
    configMapRef:
      name: wanaku
      key: wanaku.yaml
```

The operator creates these resources:

| Resource | Name | Purpose |
|----------|------|---------|
| Deployment | `rhaf-core` | One Core replica with the `Recreate` strategy. |
| PersistentVolumeClaim | `rhaf-core-data` | Core data at `/var/lib/rhaf-portal`. |
| Service | `rhaf-core-management` | Port `management`. |
| Service | `rhaf-core-mcp` | Port `mcp`. |

Core listens on `0.0.0.0` at the configured ports. The management and MCP ports
must be different. The probes use `GET /healthz` on the management port. The
operator creates no Route or Ingress.

Core runs as a non-root user. The pod uses a `RuntimeDefault` seccomp profile,
a read-only root filesystem, no Linux capabilities, no privilege escalation, and
no service account token. The CRD rejects `runAsNonRoot: false` and
`runAsUser: 0`. Set `spec.podSecurityContext` to add pod settings. On OpenShift,
leave it empty: the restricted SCC assigns the UID and `fsGroup`. On other
clusters, set `fsGroup` so Core can write to the data volume:

```yaml
spec:
  podSecurityContext:
    fsGroup: 1001
```

The management API has no caller authentication. Any pod that can reach the
`rhaf-core-management` Service can use it. Restrict access with a NetworkPolicy
if other workloads share the namespace or cluster network.

`spec.wanakuConfig` mounts one ConfigMap key read-only and passes it to Core
with `--wanaku-config`. A change to the key content restarts Core. If the
ConfigMap or key does not exist, the operator does not create or update the
Deployment and sets `ConfigValid` to `False`.

Only one installation can be active in a namespace. The installation that owns
the Core Deployment or Services stays active. If no installation runs it, the oldest
`RHAFPortalCore` becomes active. When the active installation is deleted, the
next installation waits until the previous Core pod exits before it starts Core.
During that time, `CoreAvailable` has the reason `PreviousInstallationCleanup`.
The operator sets `ConfigValid` to `False` with reason `DuplicateInstallation`
on the other installations and creates nothing for them.

The `CoreAvailable` condition reports Core readiness. While Core is not ready,
it reports the container failure reason, such as `CrashLoopBackOff`, or the
`ReconcileError` reason when the operator cannot apply a resource.

The operator creates the PVC once and does not change it later. When you delete
the `RHAFPortalCore`, Kubernetes deletes the Deployment and Services. The PVC
stays. Delete the PVC manually to remove Core data.

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
registry (see [Use the OpenShift registry](#use-the-openshift-registry)), then
deploy the Helm chart with the internal registry address:

```bash
crc start
eval "$(crc oc-env)"
oc login -u kubeadmin -p '<CRC password>'
make dev-openshift-local \
  HELM_NAMESPACE=rhaf-system \
  HELM_IMAGE=image-registry.openshift-image-registry.svc:5000/rhaf-system/rhaf-portal-operator:dev
```

### Use the OpenShift registry

The OpenShift integrated registry has two addresses:

- The external route. Use it to push images from your workstation.
- The internal service, `image-registry.openshift-image-registry.svc:5000`.
  Use it in pod specifications, such as `HELM_IMAGE` and `spec.image`.

Get the external route. OpenShift Local enables it by default. On other
clusters, enable it first:

```bash
oc patch configs.imageregistry.operator.openshift.io/cluster \
  --type merge -p '{"spec":{"defaultRoute":true}}'
REGISTRY=$(oc get route default-route -n openshift-image-registry \
  -o jsonpath='{.spec.host}')
echo "$REGISTRY"   # default-route-openshift-image-registry.apps-crc.testing
```

Create the project before you push. An image stream exists only in a project.
Log in to the registry with your OpenShift token:

```bash
oc new-project rhaf-system
podman login -u "$(oc whoami)" -p "$(oc whoami -t)" --tls-verify=false "$REGISTRY"
```

Build and push the operator and Core images. Build the Core image from a
`rhaf-portal-core` checkout:

```bash
podman build -t "$REGISTRY/rhaf-system/rhaf-portal-operator:dev" -f Containerfile .
podman push --tls-verify=false "$REGISTRY/rhaf-system/rhaf-portal-operator:dev"

podman build -t "$REGISTRY/rhaf-system/rhaf-portal-core:dev" \
  -f ../rhaf-portal-core/Containerfile ../rhaf-portal-core
podman push --tls-verify=false "$REGISTRY/rhaf-system/rhaf-portal-core:dev"
oc get imagestreams -n rhaf-system
```

Deploy the chart with the internal service address:

```bash
make dev-openshift-local \
  HELM_NAMESPACE=rhaf-system \
  HELM_IMAGE=image-registry.openshift-image-registry.svc:5000/rhaf-system/rhaf-portal-operator:dev
```

Use the same internal address for the Core image in the `RHAFPortalCore`:

```yaml
spec:
  image: image-registry.openshift-image-registry.svc:5000/rhaf-system/rhaf-portal-core:dev
```

Service accounts in `rhaf-system` can pull images from the `rhaf-system` image
streams. To pull from another project, grant the `system:image-puller` role.
`--tls-verify=false` accepts the self-signed route certificate of OpenShift Local.

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
