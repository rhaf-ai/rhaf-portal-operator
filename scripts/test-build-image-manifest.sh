#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
export IMAGE_TAG=registry.example/operator:test
export MANIFEST_CALLS="$work/calls"
export CONTAINER_TOOL=docker

cat > "$work/podman" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$MANIFEST_CALLS"
case "$1 $2" in
    'manifest rm') exit 1 ;; # No previous local manifest is a normal case.
    'manifest create')
        # Nested source indexes require all entries to be included.
        [[ $3 == --all ]] || exit 2
        [[ ${FAIL_COMMAND:-} != create ]] || exit 23
        ;;
    'manifest push') [[ ${FAIL_COMMAND:-} != push ]] || exit 24 ;;
    *) exit 2 ;;
esac
STUB
cat > "$work/docker" <<'STUB'
#!/usr/bin/env bash
echo 'Manifest publication must use Podman, even when Docker builds images.' >&2
exit 99
STUB
chmod +x "$work/podman" "$work/docker"
export PATH="$work:$PATH"

cat > "$work/expected" <<EOF
manifest rm $IMAGE_TAG
manifest create --all $IMAGE_TAG docker://$IMAGE_TAG-amd64 docker://$IMAGE_TAG-arm64
manifest push --all $IMAGE_TAG docker://$IMAGE_TAG
EOF

"$root/scripts/build-image-manifest.sh"
diff -u "$work/expected" "$MANIFEST_CALLS"

# A failed source-index import must prevent publication.
: > "$MANIFEST_CALLS"
status=0
FAIL_COMMAND=create "$root/scripts/build-image-manifest.sh" || status=$?
[[ $status == 23 ]]
head -n 2 "$work/expected" > "$work/expected-failure"
diff -u "$work/expected-failure" "$MANIFEST_CALLS"

# A failed registry push must fail the job.
: > "$MANIFEST_CALLS"
status=0
FAIL_COMMAND=push "$root/scripts/build-image-manifest.sh" || status=$?
[[ $status == 24 ]]
diff -u "$work/expected" "$MANIFEST_CALLS"

echo 'Manifest publication script tests passed.'
