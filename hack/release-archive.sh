#!/usr/bin/env bash
set -euo pipefail

version=${VERSION:?Set VERSION}
platform=${PLATFORM:?Set PLATFORM}
case "$version" in
    ''|*[!a-zA-Z0-9.+-]*) echo "Invalid version: $version" >&2; exit 1 ;;
esac
case "$platform" in
    linux-x86_64|linux-amd64) goos=linux; goarch=amd64 ;;
    linux-aarch64|linux-arm64) goos=linux; goarch=arm64 ;;
    macos-x86_64|darwin-amd64) goos=darwin; goarch=amd64 ;;
    macos-aarch64|darwin-arm64) goos=darwin; goarch=arm64 ;;
    *) echo "Unsupported platform: $platform" >&2; exit 1 ;;
esac

archive="rhaf-portal-operator-$version-$platform"
mkdir -p dist
stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT
mkdir -p "$stage/$archive"
CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" "${GO:-go}" build -trimpath -ldflags='-s -w' \
    -o "$stage/$archive/rhaf-portal-operator" ./cmd/rhaf-portal-operator
cp LICENSE README.md "$stage/$archive/"
tar -C "$stage" -czf "dist/$archive.tar.gz" "$archive"
