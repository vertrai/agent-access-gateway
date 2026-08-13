#!/bin/sh
set -eu

usage() {
  echo "Usage: $0 <agent-skills-directory>" >&2
  echo "       $0 --agent codex|claude|hermes|pi" >&2
  exit 2
}

agent_skills_dir() {
  case "$1" in
    codex)
      printf '%s\n' "${CODEX_HOME:-$HOME/.codex}/skills"
      ;;
    claude)
      printf '%s\n' "${CLAUDE_CONFIG_DIR:-$HOME/.claude}/skills"
      ;;
    hermes)
      printf '%s\n' "${HERMES_HOME:-$HOME/.hermes}/skills"
      ;;
    pi)
      printf '%s\n' "${PI_CODING_AGENT_DIR:-$HOME/.pi/agent}/skills"
      ;;
    *)
      usage
      ;;
  esac
}

if [ "$#" -eq 1 ]; then
  target_dir=$1
elif [ "$#" -eq 2 ] && [ "$1" = "--agent" ]; then
  target_dir=$(agent_skills_dir "$2")
else
  usage
fi

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
source_dir=$(CDPATH= cd -- "$script_dir/../skills" && pwd)

mkdir -p "$target_dir"
installed=0
for skill_dir in "$source_dir"/gateway-*; do
  skill_name=$(basename "$skill_dir")
  mkdir -p "$target_dir/$skill_name"
  cp -R "$skill_dir/." "$target_dir/$skill_name/"
  test -f "$target_dir/$skill_name/SKILL.md"
  installed=$((installed + 1))
  echo "Installed $skill_name -> $target_dir/$skill_name"
done

if [ "$installed" -ne 4 ]; then
  echo "Expected 4 skills, installed $installed" >&2
  exit 1
fi

if ! command -v browser-harness >/dev/null 2>&1; then
  if ! command -v uv >/dev/null 2>&1; then
    echo "uv is required to install browser-harness: https://docs.astral.sh/uv/getting-started/installation/" >&2
    exit 1
  fi
  echo "Installing required remote-browser dependency: browser-harness"
  uv tool install browser-harness
fi

browser-harness --version >/dev/null
echo "Installed and verified 4 Hub Gateway skills."
