#!/usr/bin/env bash
set -euo pipefail

project_dir=$(cd "$(dirname "$0")/.." && pwd)
source_dir="$project_dir/start-hermes"
output_dir="$project_dir/module/bin"
base_image="sandytest456/hermes-agent:linux-full"
requested_arch=${1:-${TARGET_ARCH:-}}

if [[ -n "$requested_arch" ]]; then
  arch=$requested_arch
else
  arch=$(docker image inspect "$base_image" --format '{{.Architecture}}' 2>/dev/null || true)
  if [[ -z "$arch" ]]; then
    docker pull "$base_image" >/dev/null
    arch=$(docker image inspect "$base_image" --format '{{.Architecture}}')
  fi
fi

case "$arch" in
  amd64|arm64) ;;
  *) echo "unsupported base-image architecture: $arch" >&2; exit 1 ;;
esac

mkdir -p "$output_dir"
temporary=$(mktemp "$output_dir/.start-hermes.XXXXXX")
trap 'rm -f "$temporary"' EXIT
(
  cd "$source_dir"
  go test ./...
  go vet ./...
  CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -ldflags='-s -w' -o "$temporary" ./cmd/start-hermes
)
go version -m "$temporary" >/dev/null
chmod 0755 "$temporary"
mv -f "$temporary" "$output_dir/start-hermes"
trap - EXIT
echo "built module/bin/start-hermes for linux/$arch"
