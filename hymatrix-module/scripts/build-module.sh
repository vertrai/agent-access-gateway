#!/usr/bin/env bash
set -euo pipefail

project_dir=$(cd "$(dirname "$0")/.." && pwd)
workspace_dir=${VMDOCKER_WORKSPACE_DIR:-$project_dir/tools/vmdockerv2}
agent_bin=${VMDOCKER_AGENT_BIN:-$project_dir/tools/vmdocker-agent}
profile="$project_dir/module/profile.toml"

if [[ -z "${VMDOCKER_PRIVATE_KEY:-}" ]]; then
  if ! command -v openssl >/dev/null 2>&1; then
    echo "openssl is required to generate an ephemeral module signing key" >&2
    exit 1
  fi
  VMDOCKER_PRIVATE_KEY="0x$(openssl rand -hex 32)"
  echo "Generated an ephemeral module signing key for this build (not printed or saved)." >&2
fi
if [[ ! -f "$workspace_dir/go.mod" ]]; then
  if [[ -e "$workspace_dir" ]]; then
    echo "VMDocker workspace path exists but is not a valid checkout: $workspace_dir" >&2
    exit 1
  fi
  echo "Preparing a project-local VMDocker checkout in $workspace_dir ..." >&2
  hype vmdocker get --dir "$workspace_dir"
fi
if [[ ! -x "$agent_bin" ]]; then
  echo "vmdocker-agent not found or not executable: $agent_bin" >&2
  echo "Copy a matching Linux binary to hymatrix-module/tools/vmdocker-agent or set VMDOCKER_AGENT_BIN." >&2
  exit 1
fi
if [[ ! -x "$project_dir/module/bin/start-hermes" ]]; then
  echo "start-hermes is missing; run hymatrix-module/scripts/build.sh first" >&2
  exit 1
fi

exec hype vmdocker module build \
  --dir "$workspace_dir" \
  --profile "$profile" \
  --agent-bin "$agent_bin" \
  --private-key "$VMDOCKER_PRIVATE_KEY"
