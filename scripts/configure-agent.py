#!/usr/bin/env python3
"""Persist Agent Access Gateway settings for a supported Agent host."""

import argparse
import getpass
import os
import shutil
import stat
import subprocess
import sys
import tempfile
import re
from pathlib import Path


KEYS = ("AGENT_ACCESS_GATEWAY_URL", "AGENT_ACCESS_GATEWAY_API_KEY")
HERMES_CONFLICTING_SKILLS = (
    "agentmail",
    "google-services",
    "google-workspace",
    "himalaya",
)


def hermes_env_path():
    hermes = shutil.which("hermes")
    if hermes:
        try:
            value = subprocess.check_output([hermes, "config", "env-path"], text=True, timeout=10).strip()
            if value:
                return Path(value).expanduser()
        except Exception:
            pass
    return Path(os.environ.get("HERMES_HOME", str(Path.home() / ".hermes"))) / ".env"


def quoted(value):
    return '"' + value.replace("\\", "\\\\").replace('"', '\\"') + '"'


def update_env_file(path, values):
    path.parent.mkdir(parents=True, exist_ok=True)
    original = path.read_text(encoding="utf-8").splitlines() if path.exists() else []
    output = []
    replaced = set()
    for line in original:
        stripped = line.strip()
        matched = False
        for key in KEYS:
            if stripped.startswith(key + "=") or stripped.startswith("export " + key + "="):
                output.append(f"{key}={quoted(values[key])}")
                replaced.add(key)
                matched = True
                break
        if not matched:
            output.append(line)
    if output and output[-1] != "":
        output.append("")
    for key in KEYS:
        if key not in replaced:
            output.append(f"{key}={quoted(values[key])}")
    content = "\n".join(output) + "\n"
    descriptor, temporary = tempfile.mkstemp(prefix=".agent-access-gateway-", dir=str(path.parent))
    try:
        with os.fdopen(descriptor, "w", encoding="utf-8") as handle:
            handle.write(content)
        os.chmod(temporary, stat.S_IRUSR | stat.S_IWUSR)
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)
    os.chmod(path, stat.S_IRUSR | stat.S_IWUSR)


def disable_conflicting_hermes_skills():
    try:
        from hermes_cli.config import load_config, save_config
    except ImportError as error:
        if os.environ.get("AGENT_ACCESS_GATEWAY_HERMES_REEXEC") != "1":
            hermes_python = find_hermes_python()
            if hermes_python:
                environment = os.environ.copy()
                environment["AGENT_ACCESS_GATEWAY_HERMES_REEXEC"] = "1"
                os.execve(hermes_python, [hermes_python, str(Path(__file__).resolve()), *sys.argv[1:]], environment)
        raise RuntimeError("Hermes configuration API is unavailable") from error
    config = load_config()
    skills = config.setdefault("skills", {})
    if not isinstance(skills, dict):
        raise RuntimeError("Hermes skills configuration is invalid")
    current = skills.get("disabled")
    if current is None:
        disabled = set()
    elif isinstance(current, str):
        disabled = {current.strip()} if current.strip() else set()
    else:
        try:
            disabled = {str(value).strip() for value in current if str(value).strip()}
        except TypeError as error:
            raise RuntimeError("Hermes skills.disabled configuration is invalid") from error
    disabled.update(HERMES_CONFLICTING_SKILLS)
    skills["disabled"] = sorted(disabled)

    save_config(config)


def executable_interpreter(path):
    try:
        first_line = Path(path).read_text(encoding="utf-8", errors="ignore").splitlines()[0]
    except (OSError, IndexError):
        return ""
    if first_line.startswith("#!"):
        interpreter = first_line[2:].strip().split()[0]
        if Path(interpreter).is_file() and "python" in Path(interpreter).name:
            return interpreter
    return ""


def find_hermes_python():
    hermes = shutil.which("hermes")
    if not hermes:
        return ""
    direct = executable_interpreter(hermes)
    if direct:
        return direct
    try:
        wrapper = Path(hermes).read_text(encoding="utf-8", errors="ignore")
    except OSError:
        return ""
    for target in re.findall(r'exec\s+["\']([^"\']+)["\']', wrapper):
        interpreter = executable_interpreter(target)
        if interpreter:
            return interpreter
        sibling = Path(target).parent / "python"
        if sibling.is_file():
            return str(sibling)
    return ""


def main():
    parser = argparse.ArgumentParser(description="Configure Agent Access Gateway for an Agent host")
    parser.add_argument("--agent", choices=("hermes",), required=True)
    parser.add_argument("--gateway-url", required=True)
    parser.add_argument("--env-file", default="", help=argparse.SUPPRESS)
    args = parser.parse_args()
    gateway_url = args.gateway_url.strip().rstrip("/")
    if not gateway_url.startswith(("http://", "https://")):
        raise RuntimeError("gateway URL must start with http:// or https://")
    api_key = os.environ.get("AGENT_ACCESS_GATEWAY_API_KEY_INPUT", "").strip()
    if not api_key and sys.stdin.isatty():
        api_key = getpass.getpass("Agent Access Gateway API Key: ").strip()
    if not api_key:
        raise RuntimeError("provide the API key through hidden input or AGENT_ACCESS_GATEWAY_API_KEY_INPUT")
    if not api_key.startswith("gw_sk_"):
        raise RuntimeError("gateway API key must start with gw_sk_")
    path = Path(args.env_file).expanduser() if args.env_file else hermes_env_path()
    update_env_file(path, {"AGENT_ACCESS_GATEWAY_URL": gateway_url, "AGENT_ACCESS_GATEWAY_API_KEY": api_key})
    disable_conflicting_hermes_skills()
    print(f"Configured Agent Access Gateway in {path}")
    print("The API key was stored with file mode 0600 and was not printed.")
    print("Disabled conflicting Hermes skills: agentmail, google-services, google-workspace, himalaya")
    print("Send /reload-skills in the current Hermes chat to activate the changes without restarting Hermes.")


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print(f"ERROR: {error}", file=sys.stderr)
        sys.exit(1)
