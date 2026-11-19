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

## Container images

Each cluster reads images from a different location. Minikube and OpenShift cannot
see an image in the local Podman or Docker store. Use the workflow for your target:

| Target | Command | Where the image goes |
|---|---|---|
| Local container store | `make image-local` | Podman or Docker on your workstation |
| Minikube | `make image-minikube` | The Minikube container runtime |
| OpenShift | `make image-openshift` | The OpenShift integrated registry |
| Release registry | `make build-image`, `make build-image-manifest` | A remote registry, such as Quay.io |

`make dev-minikube` and `make dev-openshift-local` build the image for their
cluster, then deploy the chart. See [Development clusters](#development-clusters).

### Build a local image

Build a single-architecture image in the local container store. Podman is the
default. Set `CONTAINER_TOOL=docker` to use Docker:

```bash
make image-local
make image-local CONTAINER_TOOL=docker
make image-local HELM_IMAGE=localhost/rhaf-portal-operator:dev
```

The image name comes from `HELM_IMAGE`. The default is
`quay.io/rhaf-ai/rhaf-portal-operator:dev`. This target does not push the image.

### Build an image for Minikube

Minikube runs its own container runtime. Build the image directly in Minikube:

```bash
minikube start
make image-minikube
```

To copy an image from the local Podman store into Minikube, save it to an archive
and load the archive:

```bash
make image-local
podman save -o /tmp/rhaf-portal-operator.tar quay.io/rhaf-ai/rhaf-portal-operator:dev
minikube image load /tmp/rhaf-portal-operator.tar
```

`make dev-minikube` sets `imagePullPolicy: IfNotPresent`. The cluster uses the
image in Minikube and does not pull it from Quay.io.

### Build and push an image to OpenShift

The OpenShift integrated registry has two addresses:

- The external route. Use it to push images from your workstation.
- The internal service, `image-registry.openshift-image-registry.svc:5000`.
  Use it in pod specifications, such as `OPENSHIFT_HELM_IMAGE` and `spec.image`.

OpenShift Local enables the external route by default. On other clusters, enable
it first:

```bash
oc patch configs.imageregistry.operator.openshift.io/cluster \
  --type merge -p '{"spec":{"defaultRoute":true}}'
```

Log in with a user token, then build and push the operator image:

```bash
oc login -u kubeadmin -p '<password>' https://api.crc.testing:6443
make image-openshift
```

`make image-openshift` runs this procedure:

1. Gets your token with `oc whoami -t`. A login with a client certificate, such
   as `system:admin`, has no token and cannot push.
2. Gets the external registry route. Set `OPENSHIFT_REGISTRY` to skip this step.
3. Creates the `HELM_NAMESPACE` project (default `rhaf-system`) if it does not
   exist. An image stream exists only in a project.
4. Logs in to the registry with the token.
5. Builds and pushes `<route>/<HELM_NAMESPACE>/rhaf-portal-operator:<OPENSHIFT_IMAGE_TAG>`.
   The default tag is `dev`.

The build uses the architecture of your workstation. If the cluster nodes use a
different architecture, set `OPENSHIFT_PLATFORM`, for example
`OPENSHIFT_PLATFORM=linux/amd64`.

To build and push the Core image from a `rhaf-portal-core` checkout, set
`CORE_DIR` to the checkout directory:

```bash
make image-openshift-core CORE_DIR=../rhaf-portal-core
oc get imagestreams -n rhaf-system
```

Use the internal address for the Core image in the `RHAFPortalCore`:

```yaml
spec:
  image: image-registry.openshift-image-registry.svc:5000/rhaf-system/rhaf-portal-core:dev
```

Service accounts in `rhaf-system` can pull images from the `rhaf-system` image
streams. To pull from another project, grant the `system:image-puller` role.

OpenShift Local uses a self-signed certificate for the route. The default
`OPENSHIFT_TLS_VERIFY=auto` disables TLS verification only for an OpenShift Local
route (`*.apps-crc.testing`). For all other routes, the targets verify the
certificate. Set `OPENSHIFT_TLS_VERIFY` to `true` or `false` to override this.

With Docker, you cannot disable TLS verification for one command. Before you use
`CONTAINER_TOOL=docker` with OpenShift Local, add the route host to
`insecure-registries` in the Docker daemon configuration, then restart Docker.
On Linux, the file is `/etc/docker/daemon.json`. On Docker Desktop, use
**Settings > Docker Engine**:

```json
{
  "insecure-registries": ["default-route-openshift-image-registry.apps-crc.testing"]
}
```

### Publish multi-architecture images

CI uses these targets to publish release images. They are not necessary for
local development.

Manifest publication requires Podman, including when Docker builds the images.
Authenticate the build tool with the registry before pushing architecture images.
Run the build command on both Linux amd64 and Linux arm64 hosts. Then authenticate
Podman and run the manifest command once after both images are available:

```bash
make build-image PUSH=true IMAGE_TAG=REGISTRY/GROUP/rhaf-portal-operator:TAG
podman login REGISTRY
make build-image-manifest IMAGE_TAG=REGISTRY/GROUP/rhaf-portal-operator:TAG
```

The build adds an architecture suffix, such as `-amd64`. The manifest publishes
the combined tag. Podman combines all entries from the architecture tags,
including tags that are themselves image indexes.

## Development clusters

The operator runs with the service-account credentials supplied by a Kubernetes
or OpenShift cluster. It reconciles finalizers for `RHAFPortalCore` and
`RHAFMCPServer`, serves metrics on port 8080, and serves health probes on port
8081.

For Minikube development, start the cluster, then build the image in Minikube and
deploy the Helm chart:

```bash
minikube start
make dev-minikube
```

For OpenShift Local development, log in, then build and push the image to the
OpenShift registry and deploy the Helm chart:

```bash
crc start
eval "$(crc oc-env)"
oc login -u kubeadmin -p '<CRC password>'
make dev-openshift-local
```

`make dev-openshift-local` deploys the image that it pushed, with the internal
registry address
`image-registry.openshift-image-registry.svc:5000/<HELM_NAMESPACE>/rhaf-portal-operator:<OPENSHIFT_IMAGE_TAG>`,
and `imagePullPolicy: Always`. It stops with an error if you set `HELM_IMAGE` or
`OPENSHIFT_HELM_IMAGE`.

To deploy an image without a build, use `make deploy-openshift-local`. Set
`OPENSHIFT_HELM_IMAGE` to select the image. `HELM_IMAGE` on the command line also
sets it. Use this target if you log in with a client certificate:

```bash
make deploy-openshift-local OPENSHIFT_HELM_IMAGE=quay.io/rhaf-ai/rhaf-portal-operator:dev
```

The development targets rebuild the image with the same tag. After the chart is
deployed, they restart the operator Deployment, so that the pod uses the new
image. The Minikube target uses `kubectl` for the restart. The OpenShift targets
use `oc`. Set `KUBECTL` or `OC` to use a different command.

Both targets install the CRDs, namespace-scoped RBAC, and a single manager
Deployment through the chart in `charts/rhaf-portal-operator`. The packaged
installation bundle is tracked in issue #8.

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
