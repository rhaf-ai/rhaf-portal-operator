#!/usr/bin/env bash
set -euo pipefail

image=${IMAGE_TAG:?Set IMAGE_TAG to the full image name and tag}
# A local manifest from a previous attempt must not affect this publication.
podman manifest rm "$image" >/dev/null 2>&1 || true
# Architecture tags can be indexes themselves, for example with attestations.
podman manifest create --all "$image" "docker://$image-amd64" "docker://$image-arm64"
podman manifest push --all "$image" "docker://$image"
