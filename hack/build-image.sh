#!/usr/bin/env bash
set -euo pipefail

tool=${CONTAINER_TOOL:-podman}
image=${IMAGE_TAG:?Set IMAGE_TAG to the full image name and tag}
case $(uname -m) in
    x86_64|amd64) arch=amd64 ;;
    aarch64|arm64) arch=arm64 ;;
    *) echo "Unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

"$tool" build --platform "linux/$arch" -f Containerfile -t "$image-$arch" .
if [[ ${PUSH:-false} == true ]]; then
    "$tool" push "$image-$arch"
fi
