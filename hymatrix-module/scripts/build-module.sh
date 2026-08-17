#!/usr/bin/env bash
set -euo pipefail

project_dir=$(cd "$(dirname "$0")/.." && pwd)
workspace_dir=${VMDOCKER_WORKSPACE_DIR:-$project_dir/tools/vmdockerv2}
workspace_ref=${VMDOCKER_WORKSPACE_REF:-main}
agent_bin=${VMDOCKER_AGENT_BIN:-$project_dir/tools/vmdocker-agent}
profile="$project_dir/module/profile.toml"
build_dir=${VMDOCKER_BUILD_DIR:-$project_dir/build}
target_arch=${1:-${TARGET_ARCH:-}}

case "$target_arch" in
  amd64|arm64) ;;
  "")
    echo "Usage: $0 <amd64|arm64>" >&2
    echo "The target architecture is required so the Module cannot silently use the Docker host default." >&2
    exit 2
    ;;
  *)
    echo "Unsupported target architecture: $target_arch (expected amd64 or arm64)" >&2
    exit 2
    ;;
esac

go_binary_setting() {
  local binary=$1
  local key=$2
  go version -m "$binary" 2>/dev/null |
    sed -n "s/^[[:space:]]*build[[:space:]]*$key=//p" |
    tail -n 1
}

if [[ -z "${VMDOCKER_PRIVATE_KEY:-}" ]]; then
  if ! command -v openssl >/dev/null 2>&1; then
    echo "openssl is required to generate an ephemeral module signing key" >&2
    exit 1
  fi
  VMDOCKER_PRIVATE_KEY="0x$(openssl rand -hex 32)"
  echo "Generated an ephemeral module signing key for this build (not printed or saved)." >&2
fi
if [[ ! -f "$workspace_dir/go.mod" ]]; then
  stale_workspace=""
  if [[ -e "$workspace_dir" ]]; then
    if [[ -d "$workspace_dir" ]] && [[ -z "$(find "$workspace_dir" -mindepth 1 -maxdepth 1 -print -quit)" ]]; then
      rmdir "$workspace_dir"
    elif [[ -d "$workspace_dir/.git" ]]; then
      stale_workspace="${workspace_dir}.incomplete.$$"
      mv "$workspace_dir" "$stale_workspace"
      echo "Moved incomplete VMDocker checkout to $stale_workspace" >&2
    else
      echo "VMDocker workspace path exists but is not a valid checkout: $workspace_dir" >&2
      echo "Remove or move that path, then retry." >&2
      exit 1
    fi
  fi
  echo "Preparing a project-local VMDocker checkout in $workspace_dir ..." >&2
  hype vmdocker get --dir "$workspace_dir" || true
  if [[ ! -f "$workspace_dir/go.mod" ]]; then
    if [[ -e "$workspace_dir" ]]; then
      failed_workspace="${workspace_dir}.failed.$$"
      mv "$workspace_dir" "$failed_workspace"
    else
      failed_workspace=""
    fi
    echo "hype did not produce a usable checkout; retrying from the GitHub source archive ..." >&2
    archive_dir=$(mktemp -d "$project_dir/tools/.vmdockerv2-download.XXXXXX")
    archive_file="$archive_dir/source.tar.gz"
    if ! curl --fail --location --silent --show-error --noproxy '*' \
      "https://github.com/cryptowizard0/vmdockerv2/archive/$workspace_ref.tar.gz" \
      --output "$archive_file" || ! tar -xzf "$archive_file" -C "$archive_dir"; then
      echo "Unable to download VMDocker. Check GitHub connectivity." >&2
      find "$archive_dir" -depth -delete
      if [[ -n "$failed_workspace" ]]; then
        find "$failed_workspace" -depth -delete
      fi
      if [[ -n "$stale_workspace" ]] && [[ ! -e "$workspace_dir" ]]; then
        mv "$stale_workspace" "$workspace_dir"
      fi
      exit 1
    fi
    extracted_dir=$(find "$archive_dir" -mindepth 1 -maxdepth 1 -type d -name 'vmdockerv2-*' -print -quit)
    if [[ -z "$extracted_dir" ]] || [[ ! -f "$extracted_dir/go.mod" ]]; then
      echo "Downloaded VMDocker archive is invalid" >&2
      find "$archive_dir" -depth -delete
      exit 1
    fi
    mv "$extracted_dir" "$workspace_dir"
    find "$archive_dir" -depth -delete
    if [[ -n "$failed_workspace" ]]; then
      find "$failed_workspace" -depth -delete
    fi
  fi
  if [[ ! -f "$workspace_dir/go.mod" ]]; then
    echo "VMDocker download completed without a usable checkout: $workspace_dir" >&2
    if [[ -n "$stale_workspace" ]] && [[ ! -e "$workspace_dir" ]]; then
      mv "$stale_workspace" "$workspace_dir"
    fi
    exit 1
  fi
  if [[ -n "$stale_workspace" ]]; then
    find "$stale_workspace" -depth -delete
  fi
fi
if [[ ! -x "$agent_bin" ]]; then
  echo "vmdocker-agent not found or not executable: $agent_bin" >&2
  echo "Copy a matching Linux binary to hymatrix-module/tools/vmdocker-agent or set VMDOCKER_AGENT_BIN." >&2
  exit 1
fi
agent_os=$(go_binary_setting "$agent_bin" GOOS)
agent_arch=$(go_binary_setting "$agent_bin" GOARCH)
if [[ "$agent_os" != "linux" ]] || [[ "$agent_arch" != "$target_arch" ]]; then
  echo "vmdocker-agent architecture mismatch: expected linux/$target_arch, got ${agent_os:-unknown}/${agent_arch:-unknown}" >&2
  echo "Rebuild it with: ./scripts/build.sh $target_arch (in the vmdockerv2_agent project), then copy it to $agent_bin" >&2
  exit 1
fi
if ! command -v docker >/dev/null 2>&1; then
  echo "Docker CLI is required to build the module artifact." >&2
  echo "Install Docker Desktop, start it, and retry." >&2
  exit 1
fi
if ! docker info >/dev/null 2>&1; then
  docker_context=$(docker context show 2>/dev/null || true)
  echo "Docker daemon is not reachable${docker_context:+ (context: $docker_context)}." >&2
  echo "Start Docker Desktop and wait until Docker reports that it is running, then retry." >&2
  echo "Diagnostic command: docker info" >&2
  exit 1
fi

echo "Building start-hermes for linux/$target_arch ..." >&2
"$project_dir/scripts/build.sh" "$target_arch"

start_hermes="$project_dir/module/bin/start-hermes"
start_os=$(go_binary_setting "$start_hermes" GOOS)
start_arch=$(go_binary_setting "$start_hermes" GOARCH)
if [[ "$start_os" != "linux" ]] || [[ "$start_arch" != "$target_arch" ]]; then
  echo "start-hermes architecture mismatch: expected linux/$target_arch, got ${start_os:-unknown}/${start_arch:-unknown}" >&2
  exit 1
fi

# The VMDocker module builder shells out to `docker build` without a platform
# flag. Docker honors this variable for FROM resolution and image metadata.
export DOCKER_DEFAULT_PLATFORM="linux/$target_arch"
echo "Building Module for $DOCKER_DEFAULT_PLATFORM ..." >&2

artifact_marker=$(mktemp "$project_dir/tools/.module-build-marker.XXXXXX")
build_log=$(mktemp "$project_dir/tools/.module-build-log.XXXXXX")
trap 'rm -f "$artifact_marker" "$build_log"' EXIT

hype vmdocker module build \
  --dir "$workspace_dir" \
  --profile "$profile" \
  --agent-bin "$agent_bin" \
  --private-key "$VMDOCKER_PRIVATE_KEY" 2>&1 | tee "$build_log"

module_image=$(sed -n 's/.*docker build started: tag=\(vmdocker-module:[^[:space:]]*\).*/\1/p' "$build_log" | tail -n 1)
if [[ -z "$module_image" ]]; then
  echo "Module build succeeded, but its Docker image tag could not be determined." >&2
  exit 1
fi
module_arch=$(docker image inspect "$module_image" --format '{{.Architecture}}')
module_os=$(docker image inspect "$module_image" --format '{{.Os}}')
if [[ "$module_os" != "linux" ]] || [[ "$module_arch" != "$target_arch" ]]; then
  echo "Module image architecture mismatch: expected linux/$target_arch, got $module_os/$module_arch" >&2
  echo "Ensure the base image in module/profile.toml provides a linux/$target_arch variant." >&2
  exit 1
fi
docker tag "$module_image" vmdocker-module:latest
echo "Module test image: vmdocker-module:latest (source: $module_image, platform: linux/$target_arch)"

artifact_source_dir="$workspace_dir/cmd/mod"
artifact_paths=()
while IFS= read -r artifact_path; do
  artifact_paths+=("$artifact_path")
done < <(find "$artifact_source_dir" -maxdepth 1 -type f -name 'mod-*.json' -newer "$artifact_marker" -print 2>/dev/null)

if [[ ${#artifact_paths[@]} -eq 0 ]]; then
  echo "Module build succeeded, but no new mod-*.json artifact was found in $artifact_source_dir" >&2
  exit 1
fi

mkdir -p "$build_dir"
for artifact_path in "${artifact_paths[@]}"; do
  artifact_target="$build_dir/$(basename "$artifact_path")"
  mv "$artifact_path" "$artifact_target"
  echo "Module artifact: $artifact_target"
done
