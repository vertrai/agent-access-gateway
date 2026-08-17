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
repository_root=$(CDPATH= cd -- "$script_dir/../.." && pwd)
source_dir="$repository_root/hymatrix-module/start-hermes/skills"
if [ ! -d "$source_dir" ]; then
  echo "Hub Gateway skill source not found: $source_dir" >&2
  exit 1
fi

mkdir -p "$target_dir"
installed=0
for skill_dir in "$source_dir"/gateway-*; do
  skill_name=$(basename "$skill_dir")
  destination="$target_dir/$skill_name"
  temporary=$(mktemp -d "$target_dir/.${skill_name}.XXXXXX")
  backup="$target_dir/.${skill_name}.previous"
  cp -R "$skill_dir/." "$temporary/"
  test -f "$temporary/SKILL.md"
  rm -rf "$backup"
  if [ -e "$destination" ]; then
    mv "$destination" "$backup"
  fi
  if ! mv "$temporary" "$destination"; then
    if [ -e "$backup" ]; then
      mv "$backup" "$destination"
    fi
    exit 1
  fi
  rm -rf "$backup"
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
