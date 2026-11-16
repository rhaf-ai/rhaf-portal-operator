#!/usr/bin/env bash
set -euo pipefail

tool=${CONTAINER_TOOL:-podman}
image=${IMAGE_TAG:?Set IMAGE_TAG to the full image name and tag}
# A local manifest from a previous attempt must not affect this publication.
"$tool" manifest rm "$image" >/dev/null 2>&1 || true
"$tool" manifest create "$image" "$image-amd64" "$image-arm64"
case "$tool" in
    *podman) "$tool" manifest push --all "$image" "docker://$image" ;;
    *docker) "$tool" manifest push "$image" ;;
    *) echo "Unsupported container tool: $tool" >&2; exit 1 ;;
esac
