#!/usr/bin/env bash
# Build an image and push it to the OpenShift integrated registry through the
# external route. Pods pull the image through the internal service address.
set -euo pipefail

tool=${CONTAINER_TOOL:-podman}
namespace=${OPENSHIFT_NAMESPACE:?Set OPENSHIFT_NAMESPACE to the target project}
name=${IMAGE_NAME:?Set IMAGE_NAME to the image repository name}
context=${BUILD_CONTEXT:?Set BUILD_CONTEXT to the build directory}
containerfile=${CONTAINERFILE:?Set CONTAINERFILE to the Containerfile path}
tag=${OPENSHIFT_IMAGE_TAG:-dev}
tls_verify=${OPENSHIFT_TLS_VERIFY:-auto}
platform=${OPENSHIFT_PLATFORM:-}
registry=${OPENSHIFT_REGISTRY:-}

if [[ ! -f $containerfile ]]; then
    echo "Containerfile not found: $containerfile" >&2
    exit 1
fi

if ! oc whoami >/dev/null 2>&1; then
    echo "oc is not logged in. Run oc login first." >&2
    exit 1
fi
# The registry accepts the OpenShift token as the password and ignores the user.
if ! token=$(oc whoami -t 2>/dev/null); then
    echo "oc whoami -t failed. Log in with a token-based user (oc login -u <user>)." >&2
    echo "Client-certificate logins cannot push. Use deploy-openshift-local to deploy only." >&2
    exit 1
fi

errors=$(mktemp)
trap 'rm -f "$errors"' EXIT

if [[ -z $registry ]]; then
    if ! registry=$(oc get route default-route -n openshift-image-registry \
        -o jsonpath='{.spec.host}' 2>"$errors"); then
        echo "Cannot read the external route of the OpenShift registry:" >&2
        sed 's/^/  /' "$errors" >&2
        cat >&2 <<'EOF'
If the route is not enabled, a cluster administrator can enable it:
  oc patch configs.imageregistry.operator.openshift.io/cluster \
    --type merge -p '{"spec":{"defaultRoute":true}}'
Or set OPENSHIFT_REGISTRY to the route host.
EOF
        exit 1
    fi
fi

case $tls_verify in
    auto)
        # Only OpenShift Local uses a self-signed route certificate by default.
        if [[ $registry == *.apps-crc.testing ]]; then tls_verify=false; else tls_verify=true; fi
        ;;
    true|false) ;;
    *) echo "OPENSHIFT_TLS_VERIFY must be auto, true, or false." >&2; exit 1 ;;
esac

# An image stream exists only in a project. A user who cannot see a project gets
# Forbidden instead of NotFound, so try to create the project in both cases.
if ! phase=$(oc get project "$namespace" -o jsonpath='{.status.phase}' 2>"$errors"); then
    if ! oc new-project "$namespace" --skip-config-write >/dev/null 2>>"$errors"; then
        echo "Cannot read or create project $namespace:" >&2
        sed 's/^/  /' "$errors" >&2
        exit 1
    fi
elif [[ $phase == Terminating ]]; then
    echo "Project $namespace is being deleted. Wait until it is gone, or use another HELM_NAMESPACE." >&2
    exit 1
fi

image="$registry/$namespace/$name:$tag"
tls_args=()
if [[ $tool == podman ]]; then
    tls_args=("--tls-verify=$tls_verify")
elif [[ $tls_verify == false ]]; then
    echo "Note: $tool cannot skip TLS verification per command." >&2
    echo "If the push fails with a certificate error, add $registry to" >&2
    echo "insecure-registries in the Docker daemon configuration." >&2
fi
build_args=()
if [[ -n $platform ]]; then
    build_args=(--platform "$platform")
fi

printf '%s' "$token" | "$tool" login ${tls_args[@]+"${tls_args[@]}"} -u openshift --password-stdin "$registry"
"$tool" build ${build_args[@]+"${build_args[@]}"} -f "$containerfile" -t "$image" "$context"
"$tool" push ${tls_args[@]+"${tls_args[@]}"} "$image"

echo "Pushed $image"
echo "Use image-registry.openshift-image-registry.svc:5000/$namespace/$name:$tag in pod specifications."
