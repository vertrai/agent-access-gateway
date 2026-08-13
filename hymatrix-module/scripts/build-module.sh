#!/usr/bin/env bash
set -euo pipefail

project_dir=$(cd "$(dirname "$0")/.." && pwd)
workspace_dir=${VMDOCKER_WORKSPACE_DIR:-/Users/sandyzhou/GolandProjects/vmdocker-workspace/vmdockerv2}
agent_bin=${VMDOCKER_AGENT_BIN:-$project_dir/tools/vmdocker-agent}
profile="$project_dir/module/profile.toml"

if [[ -z "${VMDOCKER_PRIVATE_KEY:-}" ]]; then
  echo "VMDOCKER_PRIVATE_KEY is required" >&2
  exit 1
fi
if [[ ! -d "$workspace_dir" ]]; then
  echo "VMDocker workspace not found: $workspace_dir" >&2
  exit 1
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
